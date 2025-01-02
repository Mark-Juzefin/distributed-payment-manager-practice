-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS order (
    id VARCHAR(255) PRIMARY KEY,
    user_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order;
-- +goose StatementEnd