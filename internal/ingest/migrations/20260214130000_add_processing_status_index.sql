-- +goose Up
-- +goose StatementBegin

-- Partial index for stuck row recovery: find 'processing' rows that may be orphaned.
CREATE INDEX idx_inbox_processing ON inbox(status, received_at) WHERE status = 'processing';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_inbox_processing;

-- +goose StatementEnd
