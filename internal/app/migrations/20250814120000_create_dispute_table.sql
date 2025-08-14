-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS "disputes" (
    id VARCHAR(255) PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    reason TEXT NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    opened_at TIMESTAMP NOT NULL,
    evidence_due_at TIMESTAMP NULL,
    submitted_at TIMESTAMP NULL,
    closed_at TIMESTAMP NULL,
    CONSTRAINT fk_dispute_order FOREIGN KEY (order_id) REFERENCES orders(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS disputes;
-- +goose StatementEnd