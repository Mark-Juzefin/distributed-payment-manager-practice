package order

import (
	"TestTaskJustPay/src/domain"
	"context"
)

type Repo interface {
	FindById(ctx context.Context, id string) (domain.Order, error)
	//FindAll(ctx context.Context) ([]domain.Order, error)
}
