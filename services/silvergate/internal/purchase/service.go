// Package purchase composes /purchase: validate product → authorize via acquirer
// → persist transaction → mark product as purchased → trigger capture.
package purchase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/silvergate/internal/transaction"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	products   ProductService
	authorizer Authorizer
	capturer   Capturer
	txLookup   TxLookup
	txRepo     func(postgres.Executor) transaction.Repo
	transactor postgres.Transactor
	log        *slog.Logger
}

func NewService(
	products ProductService,
	authorizer Authorizer,
	capturer Capturer,
	txLookup TxLookup,
	txRepo func(postgres.Executor) transaction.Repo,
	transactor postgres.Transactor,
	log *slog.Logger,
) *Service {
	return &Service{
		products:   products,
		authorizer: authorizer,
		capturer:   capturer,
		txLookup:   txLookup,
		txRepo:     txRepo,
		transactor: transactor,
		log:        log,
	}
}

type Request struct {
	MerchantID     string
	OrderID        string
	ProductID      uuid.UUID
	CardToken      string
	IdempotencyKey string
}

type Response struct {
	TransactionID uuid.UUID
	ProductID     uuid.UUID
	OrderID       string
	Status        transaction.Status
	Amount        int64
	Currency      string
	DeclineReason string
}

// Purchase composes a product purchase: idempotency pre-check → load product →
// authorize + persist + mark-purchased in one tx → capture outside the tx. Returns:
//   - cached Response, nil          when the idempotency key replays the same request
//   - Response{capture_pending}     when the acquirer approves and capture is kicked off
//   - Response{declined}            when the acquirer declines (no product mark, no capture)
//   - ErrProductArchived            when the product is archived
//   - ErrNotFound                   when the product does not exist for the merchant
//   - ErrIdempotencyConflict        when the key was reused for a different request
//   - ErrCapturePartiallyApplied    when authorize persisted but the follow-up capture failed
func (s *Service) Purchase(ctx context.Context, req Request) (Response, error) {
	if cached, ok, err := s.checkIdempotency(ctx, req); err != nil {
		return Response{}, err
	} else if ok {
		return cached, nil
	}

	p, err := s.products.Get(ctx, req.MerchantID, req.ProductID)
	if err != nil {
		return Response{}, err
	}
	if p.IsArchived() {
		return Response{}, ErrProductArchived
	}

	var tx *transaction.Transaction
	err = s.transactor.InTransaction(ctx, pgx.ReadCommitted, func(exec postgres.Executor) error {
		txInner, authErr := s.authorizer.AuthorizeInTx(ctx, s.txRepo(exec), transaction.AuthRequest{
			MerchantID:             req.MerchantID,
			OrderID:                req.OrderID,
			Amount:                 p.Price,
			Currency:               p.Currency,
			CardToken:              req.CardToken,
			PurchaseIdempotencyKey: req.IdempotencyKey,
			ProductID:              &req.ProductID,
		})
		if authErr != nil {
			return authErr
		}
		tx = txInner

		if tx.Status == transaction.StatusAuthorized {
			if err := s.products.MarkPurchasedInTx(ctx, exec, req.MerchantID, req.ProductID); err != nil {
				return fmt.Errorf("mark product purchased: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, transaction.ErrPurchaseIdempotencyConflict) {
			return s.resolveRace(ctx, req)
		}
		return Response{}, err
	}

	if tx.Status == transaction.StatusAuthorized {
		if _, capErr := s.capturer.Capture(ctx, transaction.CaptureRequest{
			TransactionID:  tx.ID,
			Amount:         tx.Amount,
			IdempotencyKey: tx.ID.String() + "-cap",
		}); capErr != nil {
			s.log.Error("capture failed after authorize",
				"transaction_id", tx.ID,
				"error", capErr,
			)
			return Response{
				TransactionID: tx.ID,
				ProductID:     req.ProductID,
				OrderID:       req.OrderID,
				Status:        transaction.StatusAuthorized,
				Amount:        tx.Amount,
				Currency:      tx.Currency,
			}, ErrCapturePartiallyApplied
		}
	}

	return responseFromTx(tx, req.ProductID), nil
}

// checkIdempotency looks up a prior transaction by idempotency key. Returns:
//   - not found:                  (zero, false, nil) — caller proceeds with a fresh purchase
//   - found, same request:        (cached, true, nil) — idempotent replay
//   - found, different request:   (zero, false, ErrIdempotencyConflict) — key reused for another intent
func (s *Service) checkIdempotency(ctx context.Context, req Request) (Response, bool, error) {
	existing, err := s.txLookup.GetByPurchaseIdempotencyKey(ctx, req.MerchantID, req.IdempotencyKey)
	if err != nil {
		if errors.Is(err, transaction.ErrNotFound) {
			return Response{}, false, nil
		}
		return Response{}, false, err
	}
	if !sameRequest(existing, req) {
		return Response{}, false, ErrIdempotencyConflict
	}
	productID := req.ProductID
	if existing.ProductID != nil {
		productID = *existing.ProductID
	}
	return responseFromTx(existing, productID), true, nil
}

func (s *Service) resolveRace(ctx context.Context, req Request) (Response, error) {
	existing, err := s.txLookup.GetByPurchaseIdempotencyKey(ctx, req.MerchantID, req.IdempotencyKey)
	if err != nil {
		return Response{}, fmt.Errorf("resolve idempotency race: %w", err)
	}
	if !sameRequest(existing, req) {
		return Response{}, ErrIdempotencyConflict
	}
	productID := req.ProductID
	if existing.ProductID != nil {
		productID = *existing.ProductID
	}
	return responseFromTx(existing, productID), nil
}

func sameRequest(tx *transaction.Transaction, req Request) bool {
	if tx.OrderRef != req.OrderID || tx.CardToken != req.CardToken {
		return false
	}
	if tx.ProductID == nil || *tx.ProductID != req.ProductID {
		return false
	}
	return true
}

func responseFromTx(tx *transaction.Transaction, productID uuid.UUID) Response {
	status := tx.Status
	if status == transaction.StatusAuthorized {
		status = transaction.StatusCapturePending
	}
	return Response{
		TransactionID: tx.ID,
		ProductID:     productID,
		OrderID:       tx.OrderRef,
		Status:        status,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		DeclineReason: tx.DeclineReason,
	}
}
