package webhook

import (
	"TestTaskJustPay/services/ingest/dto"
	"context"
)

// Processor defines the interface for processing webhooks.
// Implementations can handle webhooks synchronously or asynchronously.
type Processor interface {
	ProcessOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error
	ProcessDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error
	ProcessPaymentWebhook(ctx context.Context, req dto.PaymentWebhookRequest) error
}
