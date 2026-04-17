-- +goose Up
-- +goose StatementBegin

ALTER TABLE payments ADD COLUMN IF NOT EXISTS refunded_amount BIGINT NOT NULL DEFAULT 0;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed', 'voided', 'partially_refunded', 'refunded'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE payments DROP COLUMN IF EXISTS refunded_amount;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed', 'voided'));

-- +goose StatementEnd
