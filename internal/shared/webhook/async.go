package webhook

import (
	"TestTaskJustPay/internal/shared/domain/dispute"
	"TestTaskJustPay/internal/shared/domain/order"
	"TestTaskJustPay/internal/shared/messaging"
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

func (p *AsyncProcessor) ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error {
	envelope, err := messaging.NewEnvelope(webhook.UserId, "order.webhook", webhook)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	return p.orderPublisher.Publish(ctx, envelope)
}

func (p *AsyncProcessor) ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error {
	envelope, err := messaging.NewEnvelope(webhook.UserID, "dispute.webhook", webhook)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	return p.disputePublisher.Publish(ctx, envelope)
}
