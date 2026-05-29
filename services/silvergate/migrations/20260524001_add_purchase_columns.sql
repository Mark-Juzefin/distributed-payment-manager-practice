-- +goose Up
-- +goose StatementBegin

-- Add purchase context columns to transactions.
-- purchase_idempotency_key is separate from idempotency_key because the latter
-- is overwritten by Capture; a shared idempotency_keys table would supersede it.

ALTER TABLE transactions
    ADD COLUMN purchase_idempotency_key TEXT,
    ADD COLUMN product_id UUID REFERENCES products(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX idx_transactions_purchase_idempotency
    ON transactions(merchant_id, purchase_idempotency_key)
    WHERE purchase_idempotency_key IS NOT NULL;

CREATE INDEX idx_transactions_product
    ON transactions(product_id)
    WHERE product_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_transactions_product;
DROP INDEX IF EXISTS idx_transactions_purchase_idempotency;

ALTER TABLE transactions
    DROP COLUMN IF EXISTS product_id,
    DROP COLUMN IF EXISTS purchase_idempotency_key;

-- +goose StatementEnd
