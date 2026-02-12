package webhook

import (
	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"context"
)

// Processor defines the interface for processing webhooks.
// Implementations can handle webhooks synchronously or asynchronously.
type Processor interface {
	ProcessOrderUpdate(ctx context.Context, webhook order.OrderUpdate) error
	ProcessDisputeUpdate(ctx context.Context, webhook dispute.ChargebackWebhook) error
}
