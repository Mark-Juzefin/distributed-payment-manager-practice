package webhook

import (
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	"context"
)

// Processor defines the interface for processing webhooks.
// Implementations can handle webhooks synchronously or asynchronously.
type Processor interface {
	ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error
	ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error
}
