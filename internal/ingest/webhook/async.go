package webhook

import (
	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/api/messaging"
	"context"
	"fmt"
)

// AsyncProcessor processes webhooks asynchronously by publishing to Kafka.
type AsyncProcessor struct {
	orderPublisher   messaging.Publisher
	disputePublisher messaging.Publisher
}

func NewAsyncProcessor(orderPublisher, disputePublisher messaging.Publisher) *AsyncProcessor {
	return &AsyncProcessor{
		orderPublisher:   orderPublisher,
		disputePublisher: disputePublisher,
	}
}

func (p *AsyncProcessor) ProcessOrderUpdate(ctx context.Context, webhook order.OrderUpdate) error {
	envelope, err := messaging.NewEnvelope(webhook.UserId, "order.webhook", webhook)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	return p.orderPublisher.Publish(ctx, envelope)
}

func (p *AsyncProcessor) ProcessDisputeUpdate(ctx context.Context, webhook dispute.ChargebackWebhook) error {
	envelope, err := messaging.NewEnvelope(webhook.UserID, "dispute.webhook", webhook)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	return p.disputePublisher.Publish(ctx, envelope)
}
