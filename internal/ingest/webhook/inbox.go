package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"TestTaskJustPay/internal/ingest/repo/inbox"
	"TestTaskJustPay/internal/shared/dto"
)

// InboxProcessor stores webhook payloads in the inbox table for later processing.
type InboxProcessor struct {
	repo inbox.InboxRepo
}

func NewInboxProcessor(repo inbox.InboxRepo) *InboxProcessor {
	return &InboxProcessor{repo: repo}
}

func (p *InboxProcessor) ProcessOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal order update payload: %w", err)
	}

	err = p.repo.Store(ctx, inbox.NewInboxMessage{
		IdempotencyKey: "order_update:" + req.ProviderEventID,
		WebhookType:    "order_update",
		Payload:        payload,
	})
	if errors.Is(err, inbox.ErrAlreadyExists) {
		return nil // idempotent — already stored
	}
	return err
}

func (p *InboxProcessor) ProcessDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal dispute update payload: %w", err)
	}

	err = p.repo.Store(ctx, inbox.NewInboxMessage{
		IdempotencyKey: "dispute_update:" + req.ProviderEventID,
		WebhookType:    "dispute_update",
		Payload:        payload,
	})
	if errors.Is(err, inbox.ErrAlreadyExists) {
		return nil // idempotent — already stored
	}
	return err
}
