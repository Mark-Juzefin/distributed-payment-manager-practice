package order_repo

import (
	"TestTaskJustPay/src/domain"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func parseRow(row pgx.Row) (domain.Order, error) {
	var order Order

	err := row.Scan(&order.OrderID,
		&order.UserID,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt)
	if err != nil {
		return domain.Order{}, err
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, fmt.Errorf("order not found: %w", err)
	}

	return order.toDomain()
}

func parseRows(rows pgx.Rows) ([]domain.Order, error) {
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		order, err := parseRow(rows)
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
