package order_repo

import (
	domain2 "TestTaskJustPay/internal/domain"
	"github.com/google/uuid"
	"time"
)

type Order struct {
	OrderID   string
	UserID    uuid.UUID
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (m Order) toDomain() (domain2.Order, error) {
	return domain2.NewOrder(
		m.OrderID,
		m.UserID,
		m.Status,
		m.CreatedAt,
		m.UpdatedAt)
}

type Event struct {
	EventID   string
	OrderID   string
	UserID    uuid.UUID
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (m Event) toDomain() (domain2.EventBase, error) {
	return domain2.NewEventBase(
		m.EventID,
		m.OrderID,
		m.UserID,
		m.Status,
		m.CreatedAt,
		m.UpdatedAt)
}
