package order_repo

import (
	"TestTaskJustPay/src/domain"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	conn *pgxpool.Pool
}

func NewRepo(conn *pgxpool.Pool) *Repo {
	return &Repo{conn: conn}
}

func (r *Repo) FindById(ctx context.Context, id string) (domain.Order, error) {
	row := r.conn.QueryRow(ctx, "SELECT id, user_id, status, created_at, updated_at FROM orders WHERE id = $1", id)

	return parseRow(row)
}

func (r *Repo) FindByFilter(ctx context.Context, filter domain.Filter) ([]domain.Order, error) {
	rows, err := r.conn.Query(ctx, filterOrdersQuery(filter), filterOrdersArgs(filter))
	if err != nil {
		return nil, err
	}

	return parseRows(rows)
}
