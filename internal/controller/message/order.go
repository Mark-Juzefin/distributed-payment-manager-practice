package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/messaging"
	"TestTaskJustPay/pkg/logger"
)

// OrderMessageController handles order webhook messages from Kafka.
type OrderMessageController struct {
	logger  *logger.Logger
	service *order.OrderService
}

// NewOrderMessageController creates a new order message controller.
func NewOrderMessageController(l *logger.Logger, s *order.OrderService) *OrderMessageController {
	return &OrderMessageController{
		logger:  l,
		service: s,
	}
}

// HandleMessage processes a single order webhook message.
func (c *OrderMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
	var env messaging.Envelope
	if err := json.Unmarshal(value, &env); err != nil {
		c.logger.Error("Failed to unmarshal envelope: key=%s error=%v", string(key), err)
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	c.logger.Debug("Processing order message: event_id=%s key=%s type=%s",
		env.EventID, env.Key, env.Type)

	var webhook order.PaymentWebhook
	if err := json.Unmarshal(env.Payload, &webhook); err != nil {
		c.logger.Error("Failed to unmarshal webhook payload: event_id=%s error=%v", env.EventID, err)
		return fmt.Errorf("unmarshal webhook: %w", err)
	}

	if err := c.service.ProcessPaymentWebhook(ctx, webhook); err != nil {
		// Idempotency: duplicate events are not errors
		if errors.Is(err, apperror.ErrEventAlreadyStored) {
			c.logger.Info("Duplicate order event ignored: event_id=%s order_id=%s provider_event_id=%s",
				env.EventID, webhook.OrderId, webhook.ProviderEventID)
			return nil
		}

		c.logger.Error("Failed to process order webhook: event_id=%s order_id=%s error=%v",
			env.EventID, webhook.OrderId, err)
		return err
	}

	c.logger.Info("Order webhook processed: event_id=%s order_id=%s status=%s",
		env.EventID, webhook.OrderId, webhook.Status)

	return nil
}
