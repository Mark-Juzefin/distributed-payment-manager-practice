package service

import (
	"TestTaskJustPay/src/domain"
	"context"
)

type IOrderService interface {
	Get(ctx context.Context, id string) (domain.Order, error)
	Filter(ctx context.Context, filter domain.Filter) ([]domain.Order, error)
}
