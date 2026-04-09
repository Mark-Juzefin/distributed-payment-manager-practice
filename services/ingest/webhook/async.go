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
}

func NewAsyncProcessor(orderPublisher, disputePublisher messaging.Publisher) *AsyncProcessor {
	return &AsyncProcessor{
		orderPublisher:   orderPublisher,
		disputePublisher: disputePublisher,
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
