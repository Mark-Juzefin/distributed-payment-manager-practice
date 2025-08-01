-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS "order_events" (
    id VARCHAR(255) PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    user_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    received_at TIMESTAMP NOT NULL DEFAULT NOW(),
    meta JSONB
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS event;
-- +goose StatementEnd