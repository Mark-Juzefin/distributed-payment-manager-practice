package webhook

import (
	"context"

	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/ingest/apiclient"
	"TestTaskJustPay/internal/shared/dto"
)

// HTTPSyncProcessor processes webhooks synchronously by calling API service via HTTP.
type HTTPSyncProcessor struct {
	client apiclient.Client
}

// NewHTTPSyncProcessor creates a new HTTP sync processor.
func NewHTTPSyncProcessor(client apiclient.Client) *HTTPSyncProcessor {
	return &HTTPSyncProcessor{
		client: client,
	}
}

// ProcessOrderWebhook converts the webhook to a DTO and sends it to API service.
func (p *HTTPSyncProcessor) ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error {
	req := dto.OrderUpdateRequest{
		ProviderEventID: webhook.ProviderEventID,
		OrderID:         webhook.OrderId,
		UserID:          webhook.UserId,
		Status:          string(webhook.Status),
		UpdatedAt:       webhook.UpdatedAt,
		CreatedAt:       webhook.CreatedAt,
		Meta:            webhook.Meta,
	}
	return p.client.SendOrderUpdate(ctx, req)
}

// ProcessDisputeWebhook converts the webhook to a DTO and sends it to API service.
func (p *HTTPSyncProcessor) ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error {
	req := dto.DisputeUpdateRequest{
		ProviderEventID: webhook.ProviderEventID,
		OrderID:         webhook.OrderID,
		UserID:          webhook.UserID,
		Status:          string(webhook.Status),
		Reason:          webhook.Reason,
		Amount:          webhook.Amount,
		Currency:        webhook.Currency,
		OccurredAt:      webhook.OccurredAt,
		EvidenceDueAt:   webhook.EvidenceDueAt,
		Meta:            webhook.Meta,
	}
	return p.client.SendDisputeUpdate(ctx, req)
}
