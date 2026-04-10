-- +goose Up
-- +goose StatementBegin

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed', 'voided'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed'));

-- +goose StatementEnd
