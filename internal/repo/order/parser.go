package order_repo

import (
	domain2 "TestTaskJustPay/internal/domain"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func parseOrderRow(row pgx.Row) (domain2.Order, error) {
	var order Order

	err := row.Scan(&order.OrderID,
		&order.UserID,
		&order.Status,
		&order.CreatedAt,
		&order.UpdatedAt)
	if err != nil {
		return domain2.Order{}, err
	}

	return order.toDomain()
}

func parseOrderRows(rows pgx.Rows) ([]domain2.Order, error) {
	defer rows.Close()

	var orders []domain2.Order

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

func parseEventRow(row pgx.Row) (domain2.EventBase, error) {
	var event Event

	err := row.Scan(&event.EventID,
		&event.OrderID,
		&event.UserID,
		&event.Status,
		&event.CreatedAt,
		&event.UpdatedAt)
	if err != nil {
		return domain2.EventBase{}, err
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain2.EventBase{}, fmt.Errorf("event not found: %w", err)
	}

	return event.toDomain()
}

func parseEventRows(rows pgx.Rows) ([]domain2.EventBase, error) {
	defer rows.Close()

	var events []domain2.EventBase

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
