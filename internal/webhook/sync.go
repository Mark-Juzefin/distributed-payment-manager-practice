package webhook

import (
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	"context"
)

// SyncProcessor processes webhooks synchronously by calling services directly.
type SyncProcessor struct {
	orderService   *order.OrderService
	disputeService *dispute.DisputeService
}

func NewSyncProcessor(orderService *order.OrderService, disputeService *dispute.DisputeService) *SyncProcessor {
	return &SyncProcessor{
		orderService:   orderService,
		disputeService: disputeService,
	}
}

func (p *SyncProcessor) ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error {
	return p.orderService.ProcessPaymentWebhook(ctx, webhook)
}

func (p *SyncProcessor) ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error {
	return p.disputeService.ProcessChargeback(ctx, webhook)
}
