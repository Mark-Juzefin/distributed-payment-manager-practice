package ordercontroller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/paymanager/internal/order"
)

type KafkaHandler struct {
	service *order.OrderService
}

func NewKafkaHandler(s *order.OrderService) *KafkaHandler {
	return &KafkaHandler{service: s}
}

func (c *KafkaHandler) HandleMessage(ctx context.Context, key, value []byte) error {
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

	var webhook order.OrderUpdate
	if err := json.Unmarshal(env.Payload, &webhook); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal webhook payload",
			"event_id", env.EventID,
			slog.Any("error", err))
		return fmt.Errorf("unmarshal webhook: %w", err)
	}

	if err := c.service.ProcessOrderUpdate(ctx, webhook); err != nil {
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
