package order_repo

import (
	"TestTaskJustPay/src/domain"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func parseOrderRow(row pgx.Row) (domain.Order, error) {
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

func parseOrderRows(rows pgx.Rows) ([]domain.Order, error) {
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		order, err := parseOrderRow(rows)
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

func parseEventRow(row pgx.Row) (domain.EventBase, error) {
	var event Event

	err := row.Scan(&event.EventID,
		&event.OrderID,
		&event.UserID,
		&event.Status,
		&event.CreatedAt,
		&event.UpdatedAt)
	if err != nil {
		return domain.EventBase{}, err
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EventBase{}, fmt.Errorf("event not found: %w", err)
	}

	return event.toDomain()
}

func parseEventRows(rows pgx.Rows) ([]domain.EventBase, error) {
	defer rows.Close()

	var events []domain.EventBase

	for rows.Next() {
		event, err := parseEventRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}
