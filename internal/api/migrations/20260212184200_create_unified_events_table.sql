-- +goose Up
-- +goose StatementBegin

-- Unified events table for the outbox pattern (CDC source).
-- No partitioning — will be added in a separate subtask.
-- No FK — aggregate_id is generic (can be order or dispute).
CREATE TABLE IF NOT EXISTS events (
    id                 UUID         NOT NULL DEFAULT gen_random_uuid(),
    aggregate_type     VARCHAR(32)  NOT NULL,
    aggregate_id       VARCHAR(255) NOT NULL,
    event_type         VARCHAR(64)  NOT NULL,
    idempotency_key    VARCHAR(255) NOT NULL,
    payload            JSONB        NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT events_pk PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_events_idempotency
    ON events(aggregate_type, aggregate_id, idempotency_key);

CREATE INDEX IF NOT EXISTS idx_events_aggregate_lookup
    ON events(aggregate_type, aggregate_id, created_at);

CREATE INDEX IF NOT EXISTS idx_events_type_created
    ON events(event_type, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS events CASCADE;
-- +goose StatementEnd
