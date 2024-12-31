package order

import (
	"TestTaskJustPay/src/domain"
	"context"
)

type Repo interface {
	FindById(ctx context.Context, id string) (domain.Order, error)
	FindByFilter(ctx context.Context, filter domain.Filter) ([]domain.Order, error)
}
