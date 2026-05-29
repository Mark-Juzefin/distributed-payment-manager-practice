package productrepo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/silvergate/internal/product"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

const productColumns = "id, merchant_id, slug, name, description, price, currency, status, first_purchased_at, created_at, updated_at"

type PgProductRepo struct {
	db postgres.Executor
}

func NewPgProductRepo(db postgres.Executor) *PgProductRepo {
	return &PgProductRepo{db: db}
}

func (r *PgProductRepo) Create(ctx context.Context, p *product.Product) error {
	query, args, err := psql.
		Insert("products").
		Columns("id", "merchant_id", "slug", "name", "description", "price",
			"currency", "status", "first_purchased_at", "created_at", "updated_at").
		Values(p.ID, p.MerchantID, p.Slug, p.Name, p.Description, p.Price,
			p.Currency, p.Status, p.FirstPurchasedAt, p.CreatedAt, p.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert: %w", err)
	}

	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		if isUniqueViolation(err) {
			return product.ErrSlugConflict
		}
		return fmt.Errorf("exec insert: %w", err)
	}
	return nil
}

func (r *PgProductRepo) GetByID(ctx context.Context, merchantID string, id uuid.UUID) (*product.Product, error) {
	query, args, err := psql.
		Select(productColumns).
		From("products").
		Where(sq.Eq{"merchant_id": merchantID, "id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}

	p, err := scanProduct(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, product.ErrNotFound
		}
		return nil, fmt.Errorf("scan product: %w", err)
	}
	return p, nil
}

func (r *PgProductRepo) List(ctx context.Context, merchantID string, filter product.ListFilter) ([]*product.Product, *product.Cursor, error) {
	b := psql.
		Select(productColumns).
		From("products").
		Where(sq.Eq{"merchant_id": merchantID}).
		OrderBy("created_at DESC", "id DESC").
		Limit(uint64(filter.Limit) + 1)

	if filter.StatusFilter != nil {
		b = b.Where(sq.Eq{"status": *filter.StatusFilter})
	}
	if filter.Cursor != nil {
		b = b.Where(sq.Expr("(created_at, id) < (?, ?)", filter.Cursor.CreatedAt, filter.Cursor.ID))
	}

	query, args, err := b.ToSql()
	if err != nil {
		return nil, nil, fmt.Errorf("build list: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("exec list: %w", err)
	}
	defer rows.Close()

	products := make([]*product.Product, 0, filter.Limit)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan list row: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate list rows: %w", err)
	}

	var next *product.Cursor
	if len(products) > filter.Limit {
		last := products[filter.Limit-1]
		next = &product.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		products = products[:filter.Limit]
	}
	return products, next, nil
}

func (r *PgProductRepo) Update(ctx context.Context, merchantID string, id uuid.UUID, upd product.Update) error {
	b := psql.
		Update("products").
		Set("updated_at", time.Now().UTC()).
		Where(sq.Eq{"merchant_id": merchantID, "id": id, "status": product.StatusActive})

	if upd.Info != nil {
		if upd.Info.Name != nil {
			b = b.Set("name", *upd.Info.Name)
		}
		if upd.Info.Description != nil {
			b = b.Set("description", *upd.Info.Description)
		}
	}
	if upd.Locked != nil {
		if upd.Locked.Slug != nil {
			b = b.Set("slug", *upd.Locked.Slug)
		}
		if upd.Locked.Price != nil {
			b = b.Set("price", *upd.Locked.Price)
		}
		if upd.Locked.Currency != nil {
			b = b.Set("currency", *upd.Locked.Currency)
		}
		b = b.Where("first_purchased_at IS NULL")
	}

	query, args, err := b.ToSql()
	if err != nil {
		return fmt.Errorf("build update: %w", err)
	}

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return product.ErrSlugConflict
		}
		return fmt.Errorf("exec update: %w", err)
	}
	if result.RowsAffected() == 0 {
		return r.disambiguateUpdateMiss(ctx, merchantID, id, upd)
	}
	return nil
}

// disambiguateUpdateMiss is invoked when a guarded UPDATE matched no rows.
// Service already validated against a snapshot; a miss means the row was
// concurrently archived or purchased, or never existed.
func (r *PgProductRepo) disambiguateUpdateMiss(ctx context.Context, merchantID string, id uuid.UUID, upd product.Update) error {
	p, err := r.GetByID(ctx, merchantID, id)
	if err != nil {
		return err
	}
	if p.IsArchived() {
		return product.ErrArchived
	}
	if upd.Locked != nil && p.FirstPurchasedAt != nil {
		return product.ErrFieldsLocked
	}
	return product.ErrNotFound
}

func (r *PgProductRepo) SetStatus(ctx context.Context, merchantID string, id uuid.UUID, status product.Status) error {
	query, args, err := psql.
		Update("products").
		Set("status", status).
		Set("updated_at", time.Now().UTC()).
		Where(sq.Eq{"merchant_id": merchantID, "id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build set status: %w", err)
	}

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec set status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return product.ErrNotFound
	}
	return nil
}

func (r *PgProductRepo) MarkPurchased(ctx context.Context, merchantID string, id uuid.UUID) error {
	query, args, err := psql.
		Update("products").
		Set("first_purchased_at", sq.Expr("now()")).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"merchant_id": merchantID, "id": id}).
		Where("first_purchased_at IS NULL").
		ToSql()
	if err != nil {
		return fmt.Errorf("build mark purchased: %w", err)
	}

	result, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec mark purchased: %w", err)
	}
	if result.RowsAffected() == 0 {
		// Either not found or already purchased; idempotent on the latter.
		if _, err := r.GetByID(ctx, merchantID, id); err != nil {
			return err
		}
	}
	return nil
}

func scanProduct(row pgx.Row) (*product.Product, error) {
	var p product.Product
	if err := row.Scan(
		&p.ID, &p.MerchantID, &p.Slug, &p.Name, &p.Description, &p.Price,
		&p.Currency, &p.Status, &p.FirstPurchasedAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// EncodeCursor serializes a cursor to a URL-safe base64 JSON token.
// Handlers use this to publish next-page tokens to clients.
func EncodeCursor(c product.Cursor) string {
	payload := cursorPayload{CreatedAt: c.CreatedAt, ID: c.ID}
	raw, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor parses a base64 JSON token produced by EncodeCursor.
func DecodeCursor(token string) (*product.Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}
	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal cursor: %w", err)
	}
	return &product.Cursor{CreatedAt: payload.CreatedAt, ID: payload.ID}, nil
}

type cursorPayload struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}
