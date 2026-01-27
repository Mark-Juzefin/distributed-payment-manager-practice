package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/api/messaging"
)

// OrderMessageController handles order webhook messages from Kafka.
type OrderMessageController struct {
	service *order.OrderService
}

// NewOrderMessageController creates a new order message controller.
func NewOrderMessageController(s *order.OrderService) *OrderMessageController {
	return &OrderMessageController{
		service: s,
	}
}

// HandleMessage processes a single order webhook message.
func (c *OrderMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
	var env messaging.Envelope
	if err := json.Unmarshal(value, &env); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal envelope",
			"key", string(key),
			slog.Any("error", err))
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	slog.DebugContext(ctx, "Processing order message",
		"event_id", env.EventID,
		"key", env.Key,
		"type", env.Type)

	var webhook order.PaymentWebhook
	if err := json.Unmarshal(env.Payload, &webhook); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal webhook payload",
			"event_id", env.EventID,
			slog.Any("error", err))
		return fmt.Errorf("unmarshal webhook: %w", err)
	}

	if err := c.service.ProcessPaymentWebhook(ctx, webhook); err != nil {
		// Idempotency: duplicate events/orders are not errors
		if errors.Is(err, order.ErrEventAlreadyStored) {
			slog.InfoContext(ctx, "Duplicate order event ignored",
				"event_id", env.EventID,
				"order_id", webhook.OrderId,
				"provider_event_id", webhook.ProviderEventID)
			return nil
		}
		if errors.Is(err, order.ErrAlreadyExists) {
			slog.InfoContext(ctx, "Order already exists, skipping",
				"event_id", env.EventID,
				"order_id", webhook.OrderId)
			return nil
		}

		slog.ErrorContext(ctx, "Failed to process order webhook",
			"event_id", env.EventID,
			"order_id", webhook.OrderId,
			slog.Any("error", err))
		return err
	}

	slog.InfoContext(ctx, "Order webhook processed",
		"event_id", env.EventID,
		"order_id", webhook.OrderId,
		"status", webhook.Status)

	return nil
}
