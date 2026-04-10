-- +goose Up
-- +goose StatementBegin

-- Track refunded amount on transactions
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS refunded_amount BIGINT NOT NULL DEFAULT 0;

-- Update status constraint to include refund states
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed', 'voided', 'partially_refunded', 'refunded'));

CREATE TABLE refunds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    amount          BIGINT NOT NULL CHECK (amount > 0),
    status          TEXT NOT NULL CHECK (status IN ('refund_pending', 'refunded', 'refund_failed')),
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_refunds_idempotency
    ON refunds(transaction_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_refunds_transaction_id ON refunds(transaction_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS refunds;
ALTER TABLE transactions DROP COLUMN IF EXISTS refunded_amount;

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check
    CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed', 'voided'));

-- +goose StatementEnd
