package order

import (
	domain2 "TestTaskJustPay/internal/domain"
	"context"
)

type Repo interface {
	FindById(ctx context.Context, id string) (domain2.Order, error)
	FindByFilter(ctx context.Context, filter domain2.Filter) ([]domain2.Order, error)
	UpdateOrderAndSaveEvent(ctx context.Context, event domain2.Event) error
	GetEventsByOrderId(ctx context.Context, id string) ([]domain2.EventBase, error)
}
