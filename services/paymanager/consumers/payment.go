package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/paymanager/domain/events"
	"TestTaskJustPay/services/paymanager/domain/payment"
)

type PaymentMessageController struct {
	service *payment.PaymentService
}

func NewPaymentMessageController(s *payment.PaymentService) *PaymentMessageController {
	return &PaymentMessageController{service: s}
}

func (c *PaymentMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
	var env messaging.Envelope
	if err := json.Unmarshal(value, &env); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal envelope",
			"key", string(key),
			slog.Any("error", err))
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	var webhook payment.CaptureWebhook
	if err := json.Unmarshal(env.Payload, &webhook); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal payment webhook",
			"event_id", env.EventID,
			slog.Any("error", err))
		return fmt.Errorf("unmarshal webhook: %w", err)
	}

	if err := c.service.ProcessCaptureWebhook(ctx, webhook); err != nil {
		if errors.Is(err, events.ErrEventAlreadyStored) {
			slog.InfoContext(ctx, "Duplicate payment webhook ignored",
				"event_id", env.EventID,
				"transaction_id", webhook.TransactionID)
			return nil
		}
		slog.ErrorContext(ctx, "Failed to process payment webhook",
			"event_id", env.EventID,
			"transaction_id", webhook.TransactionID,
			slog.Any("error", err))
		return err
	}

	slog.InfoContext(ctx, "Payment webhook processed",
		"event_id", env.EventID,
		"transaction_id", webhook.TransactionID,
		"status", webhook.Status)

	return nil
}
