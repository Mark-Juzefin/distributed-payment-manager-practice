package order_repo

import (
	"TestTaskJustPay/src/domain"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type Order struct {
	OrderID   string    `pg:"id"`
	UserID    uuid.UUID `pg:"user_id"`
	Status    string    `pg:"status"`
	CreatedAt time.Time `pg:"created_at"`
	UpdatedAt time.Time `pg:"updated_at"`
}

func (m Order) toDomain() (domain.Order, error) {
	return domain.NewOrder(
		m.OrderID,
		m.UserID,
		m.Status,
		m.CreatedAt,
		m.UpdatedAt)
}

func newOrderFromRow(row pgx.Row) (domain.Order, error) {
	var order Order

	err := row.Scan(&order.OrderID,
		&order.UserID,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt)
	if err != nil {
		return domain.Order{}, err
	}

	fmt.Println("[]", order)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, fmt.Errorf("order not found: %w", err)
	}

	return order.toDomain()
}

type Orders []Order

func (m Orders) toDomain() ([]domain.Order, error) {
	res := make([]domain.Order, 0, len(m))

	for i, model := range m {
		data, err := domain.NewOrder(
			model.OrderID,
			model.UserID,
			model.Status,
			model.CreatedAt,
			model.UpdatedAt)
		if err != nil {
			return nil, err
		}
		res[i] = data
	}

	return res, nil
}

func newOrdersFromRows(rows pgx.Rows) ([]domain.Order, error) {
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		order, err := newOrderFromRow(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}
