package webhook

import (
	"context"

	"TestTaskJustPay/services/ingest/apiclient"
	"TestTaskJustPay/services/ingest/dto"
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

// ProcessOrderUpdate sends the order update request to API service.
func (p *HTTPSyncProcessor) ProcessOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error {
	return p.client.SendOrderUpdate(ctx, req)
}

// ProcessDisputeUpdate sends the dispute update request to API service.
func (p *HTTPSyncProcessor) ProcessDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error {
	return p.client.SendDisputeUpdate(ctx, req)
}
