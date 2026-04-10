package repo

import (
	"context"
	"errors"
	"fmt"

	"TestTaskJustPay/services/silvergate/domain/transaction"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type PgTransactionRepo struct {
	pool *pgxpool.Pool
}

func NewPgTransactionRepo(pool *pgxpool.Pool) *PgTransactionRepo {
	return &PgTransactionRepo{pool: pool}
}

func (r *PgTransactionRepo) Create(ctx context.Context, tx *transaction.Transaction) error {
	query, args, err := psql.
		Insert("transactions").
		Columns(
			"id", "merchant_id", "order_ref", "amount", "currency",
			"card_token", "status", "decline_reason", "idempotency_key",
			"created_at", "updated_at",
		).
		Values(
			tx.ID, tx.MerchantID, tx.OrderRef, tx.Amount, tx.Currency,
			tx.CardToken, tx.Status, nilIfEmpty(tx.DeclineReason), nilIfEmpty(tx.IdempotencyKey),
			tx.CreatedAt, tx.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return transaction.ErrDuplicateIdempotency
		}
		return fmt.Errorf("exec insert: %w", err)
	}
	return nil
}

func (r *PgTransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*transaction.Transaction, error) {
	query, args, err := psql.
		Select(
			"id", "merchant_id", "order_ref", "amount", "currency",
			"card_token", "status", "decline_reason", "idempotency_key",
			"refunded_amount", "created_at", "updated_at",
		).
		From("transactions").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}

	row := r.pool.QueryRow(ctx, query, args...)

	var tx transaction.Transaction
	var declineReason, idempotencyKey *string
	err = row.Scan(
		&tx.ID, &tx.MerchantID, &tx.OrderRef, &tx.Amount, &tx.Currency,
		&tx.CardToken, &tx.Status, &declineReason, &idempotencyKey,
		&tx.RefundedAmount, &tx.CreatedAt, &tx.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, transaction.ErrNotFound
		}
		return nil, fmt.Errorf("scan transaction: %w", err)
	}

	if declineReason != nil {
		tx.DeclineReason = *declineReason
	}
	if idempotencyKey != nil {
		tx.IdempotencyKey = *idempotencyKey
	}

	return &tx, nil
}

func (r *PgTransactionRepo) UpdateStatus(ctx context.Context, tx *transaction.Transaction) error {
	query, args, err := psql.
		Update("transactions").
		Set("status", tx.Status).
		Set("idempotency_key", nilIfEmpty(tx.IdempotencyKey)).
		Set("updated_at", tx.UpdatedAt).
		Where(sq.Eq{"id": tx.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update: %w", err)
	}

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return transaction.ErrDuplicateIdempotency
		}
		return fmt.Errorf("exec update: %w", err)
	}

	if result.RowsAffected() == 0 {
		return transaction.ErrNotFound
	}
	return nil
}

func (r *PgTransactionRepo) UpdateRefund(ctx context.Context, tx *transaction.Transaction) error {
	query, args, err := psql.
		Update("transactions").
		Set("status", tx.Status).
		Set("refunded_amount", tx.RefundedAmount).
		Set("updated_at", tx.UpdatedAt).
		Where(sq.Eq{"id": tx.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update refund: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec update refund: %w", err)
	}
	return nil
}

func (r *PgTransactionRepo) CreateRefund(ctx context.Context, refund *transaction.Refund) error {
	query, args, err := psql.
		Insert("refunds").
		Columns("id", "transaction_id", "amount", "status", "idempotency_key", "created_at", "updated_at").
		Values(refund.ID, refund.TransactionID, refund.Amount, refund.Status,
			nilIfEmpty(refund.IdempotencyKey), refund.CreatedAt, refund.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert refund: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return transaction.ErrDuplicateIdempotency
		}
		return fmt.Errorf("exec insert refund: %w", err)
	}
	return nil
}

func (r *PgTransactionRepo) UpdateRefundStatus(ctx context.Context, refund *transaction.Refund) error {
	query, args, err := psql.
		Update("refunds").
		Set("status", refund.Status).
		Set("updated_at", refund.UpdatedAt).
		Where(sq.Eq{"id": refund.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update refund status: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec update refund status: %w", err)
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
