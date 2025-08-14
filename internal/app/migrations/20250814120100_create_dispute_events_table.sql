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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS dispute_events;
-- +goose StatementEnd