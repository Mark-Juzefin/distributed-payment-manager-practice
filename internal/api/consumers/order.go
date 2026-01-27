package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/api/messaging"
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
		c.logger.ErrorCtx(ctx, "Failed to unmarshal envelope: key=%s error=%v", string(key), err)
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	c.logger.DebugCtx(ctx, "Processing order message: event_id=%s key=%s type=%s",
		env.EventID, env.Key, env.Type)

	var webhook order.PaymentWebhook
	if err := json.Unmarshal(env.Payload, &webhook); err != nil {
		c.logger.ErrorCtx(ctx, "Failed to unmarshal webhook payload: event_id=%s error=%v", env.EventID, err)
		return fmt.Errorf("unmarshal webhook: %w", err)
	}

	if err := c.service.ProcessPaymentWebhook(ctx, webhook); err != nil {
		// Idempotency: duplicate events/orders are not errors
		if errors.Is(err, order.ErrEventAlreadyStored) {
			c.logger.InfoCtx(ctx, "Duplicate order event ignored: event_id=%s order_id=%s provider_event_id=%s",
				env.EventID, webhook.OrderId, webhook.ProviderEventID)
			return nil
		}
		if errors.Is(err, order.ErrAlreadyExists) {
			c.logger.InfoCtx(ctx, "Order already exists, skipping: event_id=%s order_id=%s",
				env.EventID, webhook.OrderId)
			return nil
		}

		c.logger.ErrorCtx(ctx, "Failed to process order webhook: event_id=%s order_id=%s error=%v",
			env.EventID, webhook.OrderId, err)
		return err
	}

	c.logger.InfoCtx(ctx, "Order webhook processed: event_id=%s order_id=%s status=%s",
		env.EventID, webhook.OrderId, webhook.Status)

	return nil
}
