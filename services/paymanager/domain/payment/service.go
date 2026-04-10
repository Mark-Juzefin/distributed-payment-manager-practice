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
	// Parse capture delay
	var captureDelay time.Duration
	if req.CaptureDelay != "" {
		var err error
		captureDelay, err = time.ParseDuration(req.CaptureDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid capture_delay: %w", err)
		}
	}

	// 1. Authorize with Silvergate (sync)
	orderID := uuid.New()
	authResult, err := s.provider.AuthorizePayment(ctx, gateway.AuthRequest{
		MerchantID: s.merchantID,
		OrderID:    orderID.String(),
		Amount:     req.Amount,
		Currency:   req.Currency,
		CardToken:  req.CardToken,
	})
	if err != nil {
		return nil, fmt.Errorf("authorize payment: %w", err)
	}

	// 2. Create payment entity
	var p Payment
	if authResult.Status == gateway.AuthStatusAuthorized {
		p = NewAuthorized(req.Amount, req.Currency, req.CardToken, authResult.TransactionID, s.merchantID)
	} else {
		p = NewDeclined(req.Amount, req.Currency, req.CardToken, authResult.TransactionID, s.merchantID, authResult.DeclineReason)
	}

	// Set capture_at if delayed
	if p.Status == StatusAuthorized && captureDelay > 0 {
		captureAt := time.Now().UTC().Add(captureDelay)
		p.CaptureAt = &captureAt
	}

	// 3. Save to DB
	err = s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txPaymentRepo(tx)
		return txRepo.CreatePayment(ctx, p)
	})
	if err != nil {
		return nil, fmt.Errorf("save payment: %w", err)
	}

	slog.InfoContext(ctx, "payment created",
		"payment_id", p.ID,
		"status", p.Status,
		"provider_tx_id", p.ProviderTxID,
		"capture_delay", captureDelay,
	)

	// 4. If authorized — capture immediately or with delay
	if p.Status == StatusAuthorized {
		if captureDelay == 0 {
			// Instant capture
			p.Status = StatusCapturePending
			p.UpdatedAt = time.Now().UTC()
			if err := s.paymentRepo.UpdatePaymentStatus(ctx, p.ID, StatusCapturePending, ""); err != nil {
				slog.ErrorContext(ctx, "failed to mark capture_pending", "payment_id", p.ID, "error", err)
			} else {
				go s.captureInBackground(p.ID, p.ProviderTxID, p.Amount)
			}
		} else {
			// Delayed capture — goroutine waits then captures if still authorized
			go s.captureWithDelay(p.ID, p.ProviderTxID, p.Amount, captureDelay)
		}
	}

	return &p, nil
}

func (s *PaymentService) GetPaymentByID(ctx context.Context, id string) (*Payment, error) {
	return s.paymentRepo.GetPaymentByID(ctx, id)
}

func (s *PaymentService) VoidPayment(ctx context.Context, paymentID string) (*Payment, error) {
	p, err := s.paymentRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	if !p.Status.CanTransitionTo(StatusVoided) {
		return nil, ErrInvalidStatus
	}

	// Void at Silvergate (sync)
	_, err = s.provider.VoidPayment(ctx, gateway.VoidRequest{
		TransactionID: p.ProviderTxID,
	})
	if err != nil {
		return nil, fmt.Errorf("void at provider: %w", err)
	}

	// Update status
	if err := s.paymentRepo.UpdatePaymentStatus(ctx, p.ID, StatusVoided, ""); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	p.Status = StatusVoided
	p.UpdatedAt = time.Now().UTC()

	slog.InfoContext(ctx, "payment voided",
		"payment_id", p.ID,
		"provider_tx_id", p.ProviderTxID,
	)

	return p, nil
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
		case "voided":
			newStatus = StatusVoided
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

		idempotencyKey := fmt.Sprintf("webhook_%s_%s", webhook.TransactionID, webhook.Event)
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

func (s *PaymentService) captureWithDelay(paymentID, providerTxID string, amount int64, delay time.Duration) {
	time.Sleep(delay)

	ctx := context.Background()

	// Re-read payment — might have been voided while we slept
	p, err := s.paymentRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		slog.Error("delayed capture: failed to read payment", "payment_id", paymentID, "error", err)
		return
	}

	if p.Status != StatusAuthorized {
		slog.Info("delayed capture: payment no longer authorized, skipping",
			"payment_id", paymentID, "status", p.Status)
		return
	}

	// Mark capture_pending and fire capture
	if err := s.paymentRepo.UpdatePaymentStatus(ctx, p.ID, StatusCapturePending, ""); err != nil {
		slog.Error("delayed capture: failed to mark capture_pending", "payment_id", paymentID, "error", err)
		return
	}

	s.captureInBackground(paymentID, providerTxID, amount)
}
