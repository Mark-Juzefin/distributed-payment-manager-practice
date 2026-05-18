-- +goose Up
-- +goose StatementBegin

CREATE TABLE products (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id         TEXT NOT NULL,
    slug                TEXT,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    price               BIGINT NOT NULL CHECK (price > 0),
    currency            TEXT NOT NULL CHECK (length(currency) = 3),
    status              TEXT NOT NULL DEFAULT 'active',
    first_purchased_at  TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Slug unique per merchant when present; NULL slugs allowed in unlimited quantity.
CREATE UNIQUE INDEX idx_products_slug
    ON products(merchant_id, slug)
    WHERE slug IS NOT NULL;

-- Keyset pagination over (created_at DESC, id DESC) within (merchant_id, status).
CREATE INDEX idx_products_merchant_status_created
    ON products(merchant_id, status, created_at DESC, id DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS products;

-- +goose StatementEnd
