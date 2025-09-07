-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS "order_events" (
    id VARCHAR(255) PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT fk_order_event_order FOREIGN KEY (order_id) REFERENCES orders(id)
);

CREATE INDEX IF NOT EXISTS oe_kind_created_at_inc_order
    ON public.order_events (kind, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS public.oe_kind_created_at_inc_order;
DROP TABLE IF EXISTS order_events;
-- +goose StatementEnd