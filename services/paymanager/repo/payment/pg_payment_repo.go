package payment_repo

import (
	"context"
	"fmt"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/domain/payment"

	"github.com/Masterminds/squirrel"
)

type PgPaymentRepo struct {
	pg *postgres.Postgres
	repo
}

func NewPgPaymentRepo(pg *postgres.Postgres, readDB postgres.Executor) payment.PaymentRepo {
	return &PgPaymentRepo{
		pg:   pg,
		repo: repo{db: pg.Pool, readDB: readDB, builder: pg.Builder},
	}
}

func TxRepoFactory(builder squirrel.StatementBuilderType) func(postgres.Executor) payment.PaymentRepo {
	return func(tx postgres.Executor) payment.PaymentRepo {
		return &repo{db: tx, readDB: tx, builder: builder}
	}
}

type repo struct {
	db      postgres.Executor
	readDB  postgres.Executor
	builder squirrel.StatementBuilderType
}

func (r *repo) CreatePayment(ctx context.Context, p payment.Payment) error {
	query, args, err := r.builder.Insert("payments").
		Columns("id", "amount", "currency", "card_token", "status", "decline_reason",
			"provider_tx_id", "merchant_id", "capture_at", "created_at", "updated_at").
		Values(p.ID, p.Amount, p.Currency, p.CardToken, p.Status, nilIfEmpty(p.DeclineReason),
			nilIfEmpty(p.ProviderTxID), p.MerchantID, p.CaptureAt, p.CreatedAt, p.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		if postgres.IsPgErrorUniqueViolation(err) {
			return payment.ErrAlreadyExists
		}
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

func (r *repo) GetPaymentByID(ctx context.Context, id string) (*payment.Payment, error) {
	query, args, err := r.builder.
		Select("id", "amount", "currency", "card_token", "status", "decline_reason",
			"provider_tx_id", "merchant_id", "capture_at", "created_at", "updated_at").
		From("payments").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}

	return r.scanPayment(ctx, query, args...)
}

func (r *repo) GetPaymentByProviderTxID(ctx context.Context, txID string) (*payment.Payment, error) {
	query, args, err := r.builder.
		Select("id", "amount", "currency", "card_token", "status", "decline_reason",
			"provider_tx_id", "merchant_id", "capture_at", "created_at", "updated_at").
		From("payments").
		Where(squirrel.Eq{"provider_tx_id": txID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}

	return r.scanPayment(ctx, query, args...)
}

func (r *repo) UpdatePaymentStatus(ctx context.Context, id string, status payment.Status, declineReason string) error {
	q := r.builder.Update("payments").
		Set("status", status).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": id})

	if declineReason != "" {
		q = q.Set("decline_reason", declineReason)
	}

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("build update: %w", err)
	}

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	if result.RowsAffected() == 0 {
		return payment.ErrNotFound
	}
	return nil
}

func (r *repo) scanPayment(ctx context.Context, query string, args ...any) (*payment.Payment, error) {
	rows, err := r.readDB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query payment: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, payment.ErrNotFound
	}

	var p payment.Payment
	var declineReason, providerTxID *string
	err = rows.Scan(&p.ID, &p.Amount, &p.Currency, &p.CardToken, &p.Status,
		&declineReason, &providerTxID, &p.MerchantID, &p.CaptureAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan payment: %w", err)
	}

	if declineReason != nil {
		p.DeclineReason = *declineReason
	}
	if providerTxID != nil {
		p.ProviderTxID = *providerTxID
	}
	return &p, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
