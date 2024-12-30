package service

import (
	"TestTaskJustPay/src/domain"
	"context"
)

type IOrderService interface {
	Get(ctx context.Context, id string) (domain.Order, error)
	//GetAll(ctx context.Context) ([]domain.Order, error)
}
