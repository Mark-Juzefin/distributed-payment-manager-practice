-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS "dispute_events" (
    id VARCHAR(255) PRIMARY KEY,
    dispute_id VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT fk_dispute_event_dispute FOREIGN KEY (dispute_id) REFERENCES disputes(id)
);

CREATE INDEX IF NOT EXISTS de_kind_created_at_inc_dispute
    ON public.dispute_events (kind, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS public.de_kind_created_at_inc_dispute;
DROP TABLE IF EXISTS dispute_events;
-- +goose StatementEnd