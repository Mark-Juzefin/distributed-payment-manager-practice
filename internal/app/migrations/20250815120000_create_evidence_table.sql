-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS "evidence" (
    dispute_id VARCHAR(255) PRIMARY KEY,
    fields JSONB NOT NULL DEFAULT '{}',
    files JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_evidence_dispute FOREIGN KEY (dispute_id) REFERENCES disputes(id) ON DELETE CASCADE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS evidence;
-- +goose StatementEnd