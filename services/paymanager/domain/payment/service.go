package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/domain/events"
	"TestTaskJustPay/services/paymanager/domain/gateway"

	"github.com/google/uuid"
)

type PaymentService struct {
	transactor    postgres.Transactor
	txPaymentRepo func(tx postgres.Executor) PaymentRepo
	txEventStore  func(tx postgres.Executor) events.Store
	paymentRepo   PaymentRepo
	provider      gateway.Provider
	merchantID    string
}

func NewPaymentService(
	transactor postgres.Transactor,
	txPaymentRepo func(tx postgres.Executor) PaymentRepo,
	txEventStore func(tx postgres.Executor) events.Store,
	paymentRepo PaymentRepo,
	provider gateway.Provider,
	merchantID string,
) *PaymentService {
	return &PaymentService{
		transactor:    transactor,
		txPaymentRepo: txPaymentRepo,
		txEventStore:  txEventStore,
		paymentRepo:   paymentRepo,
		provider:      provider,
		merchantID:    merchantID,
	}
}

func (s *PaymentService) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*Payment, error) {
	// 1. Authorize with Silvergate (sync)
	orderId := uuid.New()
	authResult, err := s.provider.AuthorizePayment(ctx, gateway.AuthRequest{
		MerchantID: s.merchantID,
		OrderID:    orderId.String(),
		Amount:     req.Amount,
		Currency:   req.Currency,
		CardToken:  req.CardToken,
	})
	if err != nil {
		return nil, fmt.Errorf("authorize payment: %w", err)
	}

	// 2. Create payment entity based on auth result
	var p Payment
	if authResult.Status == gateway.AuthStatusAuthorized {
		p = NewAuthorized(req.Amount, req.Currency, req.CardToken, authResult.TransactionID, s.merchantID)
	} else {
		p = NewDeclined(req.Amount, req.Currency, req.CardToken, authResult.TransactionID, s.merchantID, authResult.DeclineReason)
	}

	// 3. Save to DB in transaction
	err = s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txPaymentRepo(tx)
		if err := txRepo.CreatePayment(ctx, p); err != nil {
			return fmt.Errorf("save payment: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "payment created",
		"payment_id", p.ID,
		"status", p.Status,
		"provider_tx_id", p.ProviderTxID,
	)

	// 4. If authorized, mark capture_pending and fire background capture
	if p.Status == StatusAuthorized {
		p.Status = StatusCapturePending
		p.UpdatedAt = time.Now().UTC()
		if err := s.paymentRepo.UpdatePaymentStatus(ctx, p.ID, StatusCapturePending, ""); err != nil {
			slog.ErrorContext(ctx, "failed to mark capture_pending", "payment_id", p.ID, "error", err)
		} else {
			go s.captureInBackground(p.ID, p.ProviderTxID, p.Amount)
		}
	}

	return &p, nil
}

func (s *PaymentService) GetPaymentByID(ctx context.Context, id string) (*Payment, error) {
	return s.paymentRepo.GetPaymentByID(ctx, id)
}

func (s *PaymentService) ProcessCaptureWebhook(ctx context.Context, webhook CaptureWebhook) error {
	return s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txPaymentRepo(tx)
		txEvents := s.txEventStore(tx)

		p, err := txRepo.GetPaymentByProviderTxID(ctx, webhook.TransactionID)
		if err != nil {
			return fmt.Errorf("lookup payment by provider_tx_id %s: %w", webhook.TransactionID, err)
		}

		var newStatus Status
		switch webhook.Status {
		case "captured":
			newStatus = StatusCaptured
		case "capture_failed":
			newStatus = StatusCaptureFailed
		default:
			return fmt.Errorf("unknown webhook status: %s", webhook.Status)
		}

		if !p.Status.CanTransitionTo(newStatus) {
			slog.WarnContext(ctx, "ignoring webhook for invalid transition",
				"payment_id", p.ID, "current", p.Status, "target", newStatus)
			return nil
		}

		if err := txRepo.UpdatePaymentStatus(ctx, p.ID, newStatus, ""); err != nil {
			return fmt.Errorf("update payment status: %w", err)
		}

		// Write idempotent event
		idempotencyKey := fmt.Sprintf("capture_webhook_%s_%s", webhook.TransactionID, webhook.Event)
		_, err = txEvents.CreateEvent(ctx, events.NewEvent{
			AggregateType:  "payment",
			AggregateID:    p.ID,
			EventType:      webhook.Event,
			IdempotencyKey: idempotencyKey,
			Payload:        json.RawMessage(`{}`),
			CreatedAt:      time.Now(),
		})
		if err != nil {
			return fmt.Errorf("write event: %w", err)
		}

		slog.InfoContext(ctx, "payment webhook processed",
			"payment_id", p.ID,
			"status", newStatus,
			"provider_tx_id", webhook.TransactionID,
		)

		return nil
	})
}

func (s *PaymentService) captureInBackground(paymentID, providerTxID string, amount int64) {
	ctx := context.Background()

	_, err := s.provider.CapturePayment(ctx, gateway.CaptureRequest{
		OrderID:        providerTxID,
		Amount:         float64(amount),
		IdempotencyKey: fmt.Sprintf("capture_%s", paymentID),
	})
	if err != nil {
		slog.Error("background capture failed",
			"payment_id", paymentID,
			"provider_tx_id", providerTxID,
			"error", err,
		)
	}
}
