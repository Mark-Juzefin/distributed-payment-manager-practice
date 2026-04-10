-- +goose Up
-- +goose StatementBegin

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed', 'voided'));

ALTER TABLE payments ADD COLUMN IF NOT EXISTS capture_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed'));

ALTER TABLE payments DROP COLUMN IF EXISTS capture_at;

-- +goose StatementEnd
