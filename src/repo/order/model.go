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
