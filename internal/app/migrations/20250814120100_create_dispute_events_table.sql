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

CREATE INDEX IF NOT EXISTS idx_dispute_events_created_at
    ON public.dispute_events (created_at);

CREATE INDEX IF NOT EXISTS brin_dispute_events_created_at
    ON public.dispute_events USING BRIN (created_at)
    WITH (pages_per_range = 64);

CREATE INDEX IF NOT EXISTS idx_dispute_events_dispute_id
    ON public.dispute_events (dispute_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS public.idx_dispute_events_dispute_id;
DROP INDEX IF EXISTS public.brin_dispute_events_created_at;
DROP INDEX IF EXISTS public.idx_dispute_events_created_at;
DROP TABLE IF EXISTS dispute_events;
-- +goose StatementEnd