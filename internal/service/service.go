package service

import (
	domain2 "TestTaskJustPay/internal/domain"
	"context"
)

type IOrderService interface {
	Get(ctx context.Context, id string) (domain2.Order, error)
	Filter(ctx context.Context, filter domain2.Filter) ([]domain2.Order, error)
	ProcessEvent(ctx context.Context, event domain2.Event) error
	GetEvents(ctx context.Context, orderID string) ([]domain2.EventBase, error)
}
