-- +goose Up
-- +goose StatementBegin

-- Add unique constraint for order_events idempotency
-- Ensures same provider_event_id cannot be stored twice for the same order
CREATE UNIQUE INDEX IF NOT EXISTS idx_order_events_idempotency
    ON order_events(order_id, provider_event_id);

-- Add unique constraint for dispute_events idempotency
-- Note: dispute_events is partitioned by created_at, so UNIQUE index must include partitioning column
-- Ensures same provider_event_id cannot be stored twice for the same dispute
CREATE UNIQUE INDEX IF NOT EXISTS idx_dispute_events_idempotency
    ON dispute_events(dispute_id, provider_event_id, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_order_events_idempotency;
DROP INDEX IF EXISTS idx_dispute_events_idempotency;

-- +goose StatementEnd
