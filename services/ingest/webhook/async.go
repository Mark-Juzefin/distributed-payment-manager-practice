package webhook

import (
	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/ingest/dto"
	"context"
	"fmt"
)

// AsyncProcessor processes webhooks asynchronously by publishing to Kafka.
type AsyncProcessor struct {
	orderPublisher   messaging.Publisher
	disputePublisher messaging.Publisher
	paymentPublisher messaging.Publisher
}

func NewAsyncProcessor(orderPublisher, disputePublisher, paymentPublisher messaging.Publisher) *AsyncProcessor {
	return &AsyncProcessor{
		orderPublisher:   orderPublisher,
		disputePublisher: disputePublisher,
		paymentPublisher: paymentPublisher,
	}
}

func (p *AsyncProcessor) ProcessOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error {
	envelope, err := messaging.NewEnvelope(req.UserID, "order.webhook", req)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	return p.orderPublisher.Publish(ctx, envelope)
}

func (p *AsyncProcessor) ProcessDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error {
	envelope, err := messaging.NewEnvelope(req.UserID, "dispute.webhook", req)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	return p.disputePublisher.Publish(ctx, envelope)
}

func (p *AsyncProcessor) ProcessPaymentWebhook(ctx context.Context, req dto.PaymentWebhookRequest) error {
	envelope, err := messaging.NewEnvelope(req.TransactionID, "payment.webhook", req)
	if err != nil {
		return fmt.Errorf("create envelope: %w", err)
	}
	return p.paymentPublisher.Publish(ctx, envelope)
}
