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

	return newOrderFromRow(row)
}

//func (r *Repo) FindAll(ctx context.Context) ([]domain.Order, error) {
//	var data ModelArr
//	err := r.conn.QueryRow(ctx, "SELECT * FROM orders").Scan(&data)
//	if err != nil {
//		return []domain.Order{}, nil
//	}
//	return data.toDomain()
//}
