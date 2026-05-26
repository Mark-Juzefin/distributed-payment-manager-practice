-- +goose Up
-- +goose StatementBegin

-- Subtask 2 (Feature 008): add purchase context columns to transactions.
-- See docs/features/008-products-and-checkout/spec-subtask-2.md
--
-- CAVEAT (F-α — known limitation): purchase_idempotency_key is a workaround
-- for the pre-existing design flaw where transactions.idempotency_key is
-- overwritten by Capture (transaction.MarkCapturePending mutates the field).
-- The real fix is a generic idempotency_keys table keyed by
-- (merchant_id, key, endpoint) shared across endpoints. Drop this column
-- when F-α lands.

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
