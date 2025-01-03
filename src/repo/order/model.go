package order_repo

import (
	"TestTaskJustPay/src/domain"
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

func (m Order) toDomain() (domain.Order, error) {
	return domain.NewOrder(
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

func (m Event) toDomain() (domain.EventBase, error) {
	return domain.NewEventBase(
		m.EventID,
		m.OrderID,
		m.UserID,
		m.Status,
		m.CreatedAt,
		m.UpdatedAt)
}
