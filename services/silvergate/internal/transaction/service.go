package transaction

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/silvergate/internal/acquirer"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo       Repo
	acq        acquirer.Acquirer
	webhooks   WebhookSender
	log        *slog.Logger
	transactor postgres.Transactor
	txRepo     func(postgres.Executor) Repo
}

func NewService(
	repo Repo,
	acq acquirer.Acquirer,
	webhooks WebhookSender,
	log *slog.Logger,
	transactor postgres.Transactor,
	txRepo func(postgres.Executor) Repo,
) *Service {
	return &Service{
		repo:       repo,
		acq:        acq,
		webhooks:   webhooks,
		log:        log,
		transactor: transactor,
		txRepo:     txRepo,
	}
}

type AuthRequest struct {
	MerchantID string
	OrderID    string
	Amount     int64
	Currency   string
	CardToken  string

	// Set by /purchase composition; both zero for bare /auth.
	PurchaseIdempotencyKey string
	ProductID              *uuid.UUID
}

type AuthResponse struct {
	TransactionID uuid.UUID
	Status        Status
	DeclineReason string
}

func (s *Service) Authorize(ctx context.Context, req AuthRequest) (AuthResponse, error) {
	tx, err := s.AuthorizeInTx(ctx, s.repo, req)
	if err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{
		TransactionID: tx.ID,
		Status:        tx.Status,
		DeclineReason: tx.DeclineReason,
	}, nil
}

// AuthorizeInTx runs the acquirer call and persists the transaction via the given
// repo. Callers inside an outer DB tx pass a tx-bound repo for atomicity.
func (s *Service) AuthorizeInTx(ctx context.Context, repo Repo, req AuthRequest) (*Transaction, error) {
	result, err := s.acq.Authorize(ctx, req.Amount, req.Currency, req.CardToken)
	if err != nil {
		return nil, fmt.Errorf("acquirer authorize: %w", err)
	}

	var tx *Transaction
	if result.Approved {
		tx = NewAuthorized(req.MerchantID, req.OrderID, req.Amount, req.Currency, req.CardToken)
	} else {
		tx = NewDeclined(req.MerchantID, req.OrderID, req.Amount, req.Currency, req.CardToken, result.DeclineReason)
	}

	if req.PurchaseIdempotencyKey != "" && req.ProductID != nil {
		tx.MarkProductPurchase(req.PurchaseIdempotencyKey, *req.ProductID)
	}

	if err := repo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("save transaction: %w", err)
	}

	s.log.Info("authorization processed",
		"transaction_id", tx.ID,
		"merchant_id", tx.MerchantID,
		"order_ref", tx.OrderRef,
		"status", tx.Status,
	)

	return tx, nil
}

type CaptureRequest struct {
	TransactionID  uuid.UUID
	Amount         int64
	IdempotencyKey string
}

type CaptureResponse struct {
	TransactionID uuid.UUID
	Status        Status
}

func (s *Service) Capture(ctx context.Context, req CaptureRequest) (CaptureResponse, error) {
	var tx *Transaction

	err := s.transactor.InTransaction(ctx, pgx.RepeatableRead, func(dbTx postgres.Executor) error {
		txRepo := s.txRepo(dbTx)

		var err error
		tx, err = txRepo.GetByID(ctx, req.TransactionID)
		if err != nil {
			return fmt.Errorf("get transaction: %w", err)
		}

		if err := tx.MarkCapturePending(req.IdempotencyKey); err != nil {
			return err
		}

		if err := txRepo.UpdateStatus(ctx, tx); err != nil {
			return fmt.Errorf("update transaction: %w", err)
		}

		return nil
	})
	if err != nil {
		return CaptureResponse{}, err
	}

	s.log.Info("capture initiated",
		"transaction_id", tx.ID,
		"amount", req.Amount,
	)

	// Settle asynchronously — bank processing + webhook
	go s.settleAsync(tx, req.Amount)

	return CaptureResponse{
		TransactionID: tx.ID,
		Status:        StatusCapturePending,
	}, nil
}

type RefundRequest struct {
	TransactionID  uuid.UUID
	Amount         int64
	IdempotencyKey string
}

type RefundResponse struct {
	RefundID      uuid.UUID
	TransactionID uuid.UUID
	Amount        int64
	Status        RefundStatus
}

func (s *Service) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
	var tx *Transaction
	var refund *Refund

	err := s.transactor.InTransaction(ctx, pgx.RepeatableRead, func(dbTx postgres.Executor) error {
		txRepo := s.txRepo(dbTx)

		var err error
		tx, err = txRepo.GetByID(ctx, req.TransactionID)
		if err != nil {
			return fmt.Errorf("get transaction: %w", err)
		}

		if tx.Status != StatusCaptured && tx.Status != StatusPartiallyRefunded {
			return ErrNotRefundable
		}

		remaining := tx.Amount - tx.RefundedAmount
		if req.Amount > remaining {
			return ErrRefundExceedsAmount
		}

		// Reserve refund amount within the same transaction
		tx.RefundedAmount += req.Amount
		if tx.RefundedAmount >= tx.Amount {
			tx.Status = StatusRefunded
		} else {
			tx.Status = StatusPartiallyRefunded
		}
		tx.UpdatedAt = refundNow()
		if err := txRepo.UpdateRefund(ctx, tx); err != nil {
			return fmt.Errorf("reserve refund amount: %w", err)
		}

		refund = NewRefundPending(tx.ID, req.Amount, req.IdempotencyKey)
		if err := txRepo.CreateRefund(ctx, refund); err != nil {
			return fmt.Errorf("create refund: %w", err)
		}

		return nil
	})
	if err != nil {
		return RefundResponse{}, err
	}

	s.log.Info("refund initiated",
		"refund_id", refund.ID,
		"transaction_id", tx.ID,
		"amount", req.Amount,
	)

	go s.refundAsync(tx, refund)

	return RefundResponse{
		RefundID:      refund.ID,
		TransactionID: tx.ID,
		Amount:        req.Amount,
		Status:        RefundStatusPending,
	}, nil
}

func refundNow() time.Time { return time.Now().UTC() }

func (s *Service) refundAsync(tx *Transaction, refund *Refund) {
	ctx := context.Background()

	result, err := s.acq.Refund(ctx, tx.ID.String(), refund.Amount)
	if err != nil {
		s.log.Error("acquirer refund failed", "refund_id", refund.ID, "error", err)
		refund.MarkFailed()
	} else if result.Success {
		refund.MarkRefunded()
	} else {
		s.log.Warn("refund rejected", "refund_id", refund.ID, "reason", result.Reason)
		refund.MarkFailed()
	}

	if err := s.repo.UpdateRefundStatus(ctx, refund); err != nil {
		s.log.Error("failed to update refund status", "refund_id", refund.ID, "error", err)
		return
	}

	// Acquirer rejected — release the reserved amount with retry
	if refund.Status == RefundStatusFailed {
		const maxRetries = 3
		for attempt := range maxRetries {
			err := s.repo.ReleaseRefundAmount(ctx, tx.ID, refund.Amount)
			if err == nil {
				if attempt > 0 {
					s.log.Info("refund amount released after retry",
						"refund_id", refund.ID, "attempt", attempt+1)
				}
				break
			}
			s.log.Error("failed to release refund amount",
				"refund_id", refund.ID, "attempt", attempt+1, "error", err)
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
		}
	}

	if err := s.webhooks.SendRefundResult(ctx, tx, refund); err != nil {
		s.log.Error("failed to send refund webhook", "refund_id", refund.ID, "error", err)
	}
}

type VoidResponse struct {
	TransactionID uuid.UUID
	Status        Status
}

func (s *Service) Void(ctx context.Context, txID uuid.UUID) (VoidResponse, error) {
	var tx *Transaction

	err := s.transactor.InTransaction(ctx, pgx.ReadCommitted, func(dbTx postgres.Executor) error {
		txRepo := s.txRepo(dbTx)

		var err error
		tx, err = txRepo.GetByIDForUpdate(ctx, txID)
		if err != nil {
			return fmt.Errorf("get transaction: %w", err)
		}

		if err := tx.MarkVoided(); err != nil {
			return err
		}

		// Void with bank (sync) — row is locked, no concurrent capture can proceed
		result, err := s.acq.Void(ctx, tx.ID.String())
		if err != nil {
			return fmt.Errorf("acquirer void: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("void rejected: %s", result.Reason)
		}

		if err := txRepo.UpdateStatus(ctx, tx); err != nil {
			return fmt.Errorf("update transaction: %w", err)
		}

		return nil
	})
	if err != nil {
		return VoidResponse{}, err
	}

	s.log.Info("transaction voided", "transaction_id", tx.ID)

	go func() {
		if err := s.webhooks.SendCaptureResult(context.Background(), tx); err != nil {
			s.log.Error("failed to send void webhook", "transaction_id", tx.ID, "error", err)
		}
	}()

	return VoidResponse{
		TransactionID: tx.ID,
		Status:        StatusVoided,
	}, nil
}

func (s *Service) settleAsync(tx *Transaction, amount int64) {
	ctx := context.Background()

	result, err := s.acq.Settle(ctx, tx.ID.String(), amount)

	var nextStatus Status
	if err != nil {
		s.log.Error("acquirer settle failed", "transaction_id", tx.ID, "error", err)
		nextStatus = StatusCaptureFailed
	} else if result.Success {
		nextStatus = StatusCaptured
	} else {
		s.log.Warn("settlement rejected", "transaction_id", tx.ID, "reason", result.Reason)
		nextStatus = StatusCaptureFailed
	}

	if err := s.repo.CompareAndUpdateStatus(ctx, tx.ID, StatusCapturePending, nextStatus); err != nil {
		s.log.Error("failed to update transaction after settle", "transaction_id", tx.ID, "error", err)
		return
	}

	if err := s.webhooks.SendCaptureResult(ctx, tx); err != nil {
		s.log.Error("failed to send webhook", "transaction_id", tx.ID, "error", err)
	}
}
