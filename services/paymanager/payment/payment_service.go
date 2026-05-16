package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/events"
	"TestTaskJustPay/services/paymanager/gateway"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Provider is the minimal interface this domain requires from the payment gateway.
type Provider interface {
	AuthorizePayment(ctx context.Context, req gateway.AuthRequest) (gateway.AuthResult, error)
	CapturePayment(ctx context.Context, req gateway.CaptureRequest) (gateway.CaptureResult, error)
	VoidPayment(ctx context.Context, req gateway.VoidRequest) (gateway.VoidResult, error)
	RefundPayment(ctx context.Context, req gateway.RefundRequest) (gateway.RefundResult, error)
}

type PaymentService struct {
	transactor    postgres.Transactor
	txPaymentRepo func(tx postgres.Executor) PaymentRepo
	txEventStore  func(tx postgres.Executor) events.Store
	paymentRepo   PaymentRepo
	provider      Provider
	merchantID    string
}

func NewPaymentService(
	transactor postgres.Transactor,
	txPaymentRepo func(tx postgres.Executor) PaymentRepo,
	txEventStore func(tx postgres.Executor) events.Store,
	paymentRepo PaymentRepo,
	provider Provider,
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
	var captureDelay time.Duration
	if req.CaptureDelay != "" {
		var err error
		captureDelay, err = time.ParseDuration(req.CaptureDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid capture_delay: %w", err)
		}
	}

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

	var p Payment
	if authResult.Status == gateway.AuthStatusAuthorized {
		p = NewAuthorized(req.Amount, req.Currency, req.CardToken, authResult.TransactionID, s.merchantID)
	} else {
		p = NewDeclined(req.Amount, req.Currency, req.CardToken, authResult.TransactionID, s.merchantID, authResult.DeclineReason)
	}

	if p.Status == StatusAuthorized && captureDelay > 0 {
		captureAt := time.Now().UTC().Add(captureDelay)
		p.CaptureAt = &captureAt
	}

	err = s.transactor.InTransaction(ctx, pgx.RepeatableRead, func(tx postgres.Executor) error {
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

	if p.Status == StatusAuthorized {
		if captureDelay == 0 {
			p.Status = StatusCapturePending
			p.UpdatedAt = time.Now().UTC()
			if err := s.paymentRepo.UpdatePaymentStatus(ctx, p.ID, StatusCapturePending, ""); err != nil {
				slog.ErrorContext(ctx, "failed to mark capture_pending", "payment_id", p.ID, "error", err)
			} else {
				go s.captureInBackground(p.ID, p.ProviderTxID, p.Amount)
			}
		} else {
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

	_, err = s.provider.VoidPayment(ctx, gateway.VoidRequest{
		TransactionID: p.ProviderTxID,
	})
	if err != nil {
		return nil, fmt.Errorf("void at provider: %w", err)
	}

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

func (s *PaymentService) RefundPayment(ctx context.Context, paymentID string, req RefundRequest) (*Payment, error) {
	p, err := s.paymentRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	if p.Status != StatusCaptured && p.Status != StatusPartiallyRefunded {
		return nil, ErrInvalidStatus
	}

	remaining := p.Amount - p.RefundedAmount
	if req.Amount > remaining {
		return nil, ErrRefundExceedsAmount
	}

	_, err = s.provider.RefundPayment(ctx, gateway.RefundRequest{
		TransactionID:  p.ProviderTxID,
		Amount:         req.Amount,
		IdempotencyKey: fmt.Sprintf("refund_%s_%d", paymentID, req.Amount),
	})
	if err != nil {
		return nil, fmt.Errorf("refund at provider: %w", err)
	}

	slog.InfoContext(ctx, "refund initiated",
		"payment_id", p.ID,
		"amount", req.Amount,
		"provider_tx_id", p.ProviderTxID,
	)

	return p, nil
}

func (s *PaymentService) ProcessCaptureWebhook(ctx context.Context, webhook CaptureWebhook) error {
	return s.transactor.InTransaction(ctx, pgx.RepeatableRead, func(tx postgres.Executor) error {
		txRepo := s.txPaymentRepo(tx)
		txEvents := s.txEventStore(tx)

		p, err := txRepo.GetPaymentByProviderTxID(ctx, webhook.TransactionID)
		if err != nil {
			return fmt.Errorf("lookup payment by provider_tx_id %s: %w", webhook.TransactionID, err)
		}

		if webhook.Event == "transaction.refunded" {
			p.RefundedAmount += webhook.Amount
			var newStatus Status
			if p.RefundedAmount >= p.Amount {
				newStatus = StatusRefunded
			} else {
				newStatus = StatusPartiallyRefunded
			}
			if err := txRepo.UpdatePaymentRefund(ctx, p.ID, newStatus, p.RefundedAmount); err != nil {
				return fmt.Errorf("update payment refund: %w", err)
			}

			idempotencyKey := fmt.Sprintf("webhook_%s_%s_%s", webhook.TransactionID, webhook.Event, webhook.RefundID)
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

			slog.InfoContext(ctx, "payment refund webhook processed",
				"payment_id", p.ID,
				"status", newStatus,
				"refunded_amount", p.RefundedAmount,
			)
			return nil
		}

		var newStatus Status
		switch webhook.Status {
		case "captured":
			newStatus = StatusCaptured
		case "capture_failed":
			newStatus = StatusCaptureFailed
		case "voided":
			newStatus = StatusVoided
		case "refund_failed":
			slog.WarnContext(ctx, "refund failed at provider",
				"payment_id", p.ID, "transaction_id", webhook.TransactionID)
			return nil
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

	if err := s.paymentRepo.UpdatePaymentStatus(ctx, p.ID, StatusCapturePending, ""); err != nil {
		slog.Error("delayed capture: failed to mark capture_pending", "payment_id", paymentID, "error", err)
		return
	}

	s.captureInBackground(paymentID, providerTxID, amount)
}
