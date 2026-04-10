package transaction

import (
	"context"
	"fmt"
	"log/slog"

	"TestTaskJustPay/services/silvergate/acquirer"
	"github.com/google/uuid"
)

type WebhookSender interface {
	SendCaptureResult(ctx context.Context, tx *Transaction) error
	SendRefundResult(ctx context.Context, tx *Transaction, refund *Refund) error
}

type Service struct {
	repo     Repo
	acq      acquirer.Acquirer
	webhooks WebhookSender
	log      *slog.Logger
}

func NewService(repo Repo, acq acquirer.Acquirer, webhooks WebhookSender, log *slog.Logger) *Service {
	return &Service{
		repo:     repo,
		acq:      acq,
		webhooks: webhooks,
		log:      log,
	}
}

type AuthRequest struct {
	MerchantID string
	OrderID    string
	Amount     int64
	Currency   string
	CardToken  string
}

type AuthResponse struct {
	TransactionID uuid.UUID
	Status        Status
	DeclineReason string
}

func (s *Service) Authorize(ctx context.Context, req AuthRequest) (AuthResponse, error) {
	result, err := s.acq.Authorize(ctx, req.Amount, req.Currency, req.CardToken)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("acquirer authorize: %w", err)
	}

	var tx *Transaction
	if result.Approved {
		tx = NewAuthorized(req.MerchantID, req.OrderID, req.Amount, req.Currency, req.CardToken)
	} else {
		tx = NewDeclined(req.MerchantID, req.OrderID, req.Amount, req.Currency, req.CardToken, result.DeclineReason)
	}

	if err := s.repo.Create(ctx, tx); err != nil {
		return AuthResponse{}, fmt.Errorf("save transaction: %w", err)
	}

	s.log.Info("authorization processed",
		"transaction_id", tx.ID,
		"merchant_id", tx.MerchantID,
		"order_ref", tx.OrderRef,
		"status", tx.Status,
	)

	return AuthResponse{
		TransactionID: tx.ID,
		Status:        tx.Status,
		DeclineReason: tx.DeclineReason,
	}, nil
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
	tx, err := s.repo.GetByID(ctx, req.TransactionID)
	if err != nil {
		return CaptureResponse{}, fmt.Errorf("get transaction: %w", err)
	}

	if err := tx.MarkCapturePending(req.IdempotencyKey); err != nil {
		return CaptureResponse{}, err
	}

	if err := s.repo.UpdateStatus(ctx, tx); err != nil {
		return CaptureResponse{}, fmt.Errorf("update transaction: %w", err)
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
	tx, err := s.repo.GetByID(ctx, req.TransactionID)
	if err != nil {
		return RefundResponse{}, fmt.Errorf("get transaction: %w", err)
	}

	// Validate refundable state
	if tx.Status != StatusCaptured && tx.Status != StatusPartiallyRefunded {
		return RefundResponse{}, ErrNotRefundable
	}

	// Validate amount
	remaining := tx.Amount - tx.RefundedAmount
	if req.Amount > remaining {
		return RefundResponse{}, ErrRefundExceedsAmount
	}

	// Create refund record
	refund := NewRefundPending(tx.ID, req.Amount, req.IdempotencyKey)
	if err := s.repo.CreateRefund(ctx, refund); err != nil {
		return RefundResponse{}, fmt.Errorf("create refund: %w", err)
	}

	s.log.Info("refund initiated",
		"refund_id", refund.ID,
		"transaction_id", tx.ID,
		"amount", req.Amount,
	)

	// Process refund async
	go s.refundAsync(tx, refund)

	return RefundResponse{
		RefundID:      refund.ID,
		TransactionID: tx.ID,
		Amount:        req.Amount,
		Status:        RefundStatusPending,
	}, nil
}

func (s *Service) refundAsync(tx *Transaction, refund *Refund) {
	ctx := context.Background()

	result, err := s.acq.Refund(ctx, tx.ID.String(), refund.Amount)
	if err != nil {
		s.log.Error("acquirer refund failed", "refund_id", refund.ID, "error", err)
		refund.MarkFailed()
	} else if result.Success {
		refund.MarkRefunded()
		tx.RefundedAmount += refund.Amount
		tx.UpdatedAt = refund.UpdatedAt
		if tx.RefundedAmount >= tx.Amount {
			tx.Status = StatusRefunded
		} else {
			tx.Status = StatusPartiallyRefunded
		}
	} else {
		s.log.Warn("refund rejected", "refund_id", refund.ID, "reason", result.Reason)
		refund.MarkFailed()
	}

	if err := s.repo.UpdateRefundStatus(ctx, refund); err != nil {
		s.log.Error("failed to update refund status", "refund_id", refund.ID, "error", err)
		return
	}

	if refund.Status == RefundStatusDone {
		if err := s.repo.UpdateRefund(ctx, tx); err != nil {
			s.log.Error("failed to update transaction refund", "transaction_id", tx.ID, "error", err)
			return
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
	tx, err := s.repo.GetByID(ctx, txID)
	if err != nil {
		return VoidResponse{}, fmt.Errorf("get transaction: %w", err)
	}

	if err := tx.MarkVoided(); err != nil {
		return VoidResponse{}, err
	}

	// Void with bank (sync — void is fast)
	result, err := s.acq.Void(ctx, tx.ID.String())
	if err != nil {
		return VoidResponse{}, fmt.Errorf("acquirer void: %w", err)
	}
	if !result.Success {
		return VoidResponse{}, fmt.Errorf("void rejected: %s", result.Reason)
	}

	if err := s.repo.UpdateStatus(ctx, tx); err != nil {
		return VoidResponse{}, fmt.Errorf("update transaction: %w", err)
	}

	s.log.Info("transaction voided", "transaction_id", tx.ID)

	// Send webhook async
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
	if err != nil {
		s.log.Error("acquirer settle failed", "transaction_id", tx.ID, "error", err)
		_ = tx.MarkCaptureFailed()
	} else if result.Success {
		_ = tx.MarkCaptured()
	} else {
		s.log.Warn("settlement rejected", "transaction_id", tx.ID, "reason", result.Reason)
		_ = tx.MarkCaptureFailed()
	}

	if err := s.repo.UpdateStatus(ctx, tx); err != nil {
		s.log.Error("failed to update transaction after settle", "transaction_id", tx.ID, "error", err)
		return
	}

	if err := s.webhooks.SendCaptureResult(ctx, tx); err != nil {
		s.log.Error("failed to send webhook", "transaction_id", tx.ID, "error", err)
	}
}
