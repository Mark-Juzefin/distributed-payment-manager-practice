-- +goose Up
-- +goose StatementBegin

CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    amount          BIGINT NOT NULL,
    currency        TEXT NOT NULL CHECK (length(currency) = 3),
    card_token      TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('authorized', 'declined', 'capture_pending', 'captured', 'capture_failed')),
    decline_reason  TEXT,
    provider_tx_id  TEXT,
    merchant_id     TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_provider_tx_id ON payments(provider_tx_id) WHERE provider_tx_id IS NOT NULL;
CREATE INDEX idx_payments_status ON payments(status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS payments;

-- +goose StatementEnd
