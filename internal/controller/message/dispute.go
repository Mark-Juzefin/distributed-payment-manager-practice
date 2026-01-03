package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"TestTaskJustPay/internal/shared/domain/dispute"
	"TestTaskJustPay/internal/shared/messaging"
	"TestTaskJustPay/pkg/logger"
)

// DisputeMessageController handles dispute/chargeback messages from Kafka.
type DisputeMessageController struct {
	logger  *logger.Logger
	service *dispute.DisputeService
}

// NewDisputeMessageController creates a new dispute message controller.
func NewDisputeMessageController(l *logger.Logger, s *dispute.DisputeService) *DisputeMessageController {
	return &DisputeMessageController{
		logger:  l,
		service: s,
	}
}

// HandleMessage processes a single dispute/chargeback message.
func (c *DisputeMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
	var env messaging.Envelope
	if err := json.Unmarshal(value, &env); err != nil {
		c.logger.Error("Failed to unmarshal envelope: key=%s error=%v", string(key), err)
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	c.logger.Debug("Processing dispute message: event_id=%s key=%s type=%s",
		env.EventID, env.Key, env.Type)

	var webhook dispute.ChargebackWebhook
	if err := json.Unmarshal(env.Payload, &webhook); err != nil {
		c.logger.Error("Failed to unmarshal webhook payload: event_id=%s error=%v", env.EventID, err)
		return fmt.Errorf("unmarshal webhook: %w", err)
	}

	if err := c.service.ProcessChargeback(ctx, webhook); err != nil {
		// Idempotency: duplicate events are not errors
		if errors.Is(err, dispute.ErrEventAlreadyStored) {
			c.logger.Info("Duplicate dispute event ignored: event_id=%s user_id=%s order_id=%s provider_event_id=%s",
				env.EventID, webhook.UserID, webhook.OrderID, webhook.ProviderEventID)
			return nil
		}

		c.logger.Error("Failed to process chargeback webhook: event_id=%s user_id=%s order_id=%s error=%v",
			env.EventID, webhook.UserID, webhook.OrderID, err)
		return err
	}

	c.logger.Info("Chargeback webhook processed: event_id=%s user_id=%s order_id=%s status=%s",
		env.EventID, webhook.UserID, webhook.OrderID, webhook.Status)

	return nil
}
