package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/paymanager/dispute"
)

type KafkaHandler struct {
	service *dispute.DisputeService
}

func NewKafkaHandler(s *dispute.DisputeService) *KafkaHandler {
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
		if errors.Is(err, dispute.ErrEventAlreadyStored) {
			slog.InfoContext(ctx, "Duplicate dispute event ignored",
				"event_id", env.EventID,
				"order_id", webhook.OrderID,
				"provider_event_id", webhook.ProviderEventID)
			return nil
		}

		slog.ErrorContext(ctx, "Failed to process chargeback webhook",
			"event_id", env.EventID,
			"order_id", webhook.OrderID,
			slog.Any("error", err))
		return err
	}

	slog.InfoContext(ctx, "Chargeback webhook processed",
		"event_id", env.EventID,
		"order_id", webhook.OrderID,
		"status", webhook.Status)

	return nil
}
