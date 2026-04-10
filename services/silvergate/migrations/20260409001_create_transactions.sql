-- +goose Up
-- +goose StatementBegin

CREATE TABLE transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id     TEXT NOT NULL,
    order_ref       TEXT NOT NULL,
    amount          BIGINT NOT NULL,
    currency        TEXT NOT NULL CHECK (length(currency) = 3),
    card_token      TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed', 'voided')),
    decline_reason  TEXT,
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_transactions_idempotency
    ON transactions(merchant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_transactions_merchant_order
    ON transactions(merchant_id, order_ref);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS transactions;

-- +goose StatementEnd
