-- +goose Up
-- +goose StatementBegin

CREATE TABLE inbox (
    id               UUID         NOT NULL DEFAULT gen_random_uuid(),
    idempotency_key  VARCHAR(512) NOT NULL,
    webhook_type     VARCHAR(64)  NOT NULL,
    payload          JSONB        NOT NULL,
    status           VARCHAR(32)  NOT NULL DEFAULT 'pending',
    received_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    processed_at     TIMESTAMPTZ,
    error_message    TEXT,
    retry_count      INT          NOT NULL DEFAULT 0,
    CONSTRAINT inbox_pk PRIMARY KEY (id)
);

CREATE UNIQUE INDEX idx_inbox_idempotency ON inbox(idempotency_key);
CREATE INDEX idx_inbox_pending ON inbox(status, received_at) WHERE status = 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS inbox;

-- +goose StatementEnd
