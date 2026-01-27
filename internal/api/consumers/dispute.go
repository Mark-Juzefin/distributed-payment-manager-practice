package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/messaging"
)

// DisputeMessageController handles dispute/chargeback messages from Kafka.
type DisputeMessageController struct {
	service *dispute.DisputeService
}

// NewDisputeMessageController creates a new dispute message controller.
func NewDisputeMessageController(s *dispute.DisputeService) *DisputeMessageController {
	return &DisputeMessageController{
		service: s,
	}
}

// HandleMessage processes a single dispute/chargeback message.
func (c *DisputeMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
	var env messaging.Envelope
	if err := json.Unmarshal(value, &env); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal envelope",
			"key", string(key),
			slog.Any("error", err))
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	slog.DebugContext(ctx, "Processing dispute message",
		"event_id", env.EventID,
		"key", env.Key,
		"type", env.Type)

	var webhook dispute.ChargebackWebhook
	if err := json.Unmarshal(env.Payload, &webhook); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal webhook payload",
			"event_id", env.EventID,
			slog.Any("error", err))
		return fmt.Errorf("unmarshal webhook: %w", err)
	}

	if err := c.service.ProcessChargeback(ctx, webhook); err != nil {
		// Idempotency: duplicate events are not errors
		if errors.Is(err, dispute.ErrEventAlreadyStored) {
			slog.InfoContext(ctx, "Duplicate dispute event ignored",
				"event_id", env.EventID,
				"user_id", webhook.UserID,
				"order_id", webhook.OrderID,
				"provider_event_id", webhook.ProviderEventID)
			return nil
		}

		slog.ErrorContext(ctx, "Failed to process chargeback webhook",
			"event_id", env.EventID,
			"user_id", webhook.UserID,
			"order_id", webhook.OrderID,
			slog.Any("error", err))
		return err
	}

	slog.InfoContext(ctx, "Chargeback webhook processed",
		"event_id", env.EventID,
		"user_id", webhook.UserID,
		"order_id", webhook.OrderID,
		"status", webhook.Status)

	return nil
}
