package order

import (
	"TestTaskJustPay/src/domain"
	"TestTaskJustPay/src/service"
	"context"
)

type orderService struct {
	orderRepo Repo
}

func NewOrderService(orderRepo Repo) service.IOrderService {
	return &orderService{orderRepo}
}

func (s *orderService) Get(ctx context.Context, id string) (domain.Order, error) {
	return s.orderRepo.FindById(ctx, id)
}

func (s *orderService) Filter(ctx context.Context, filter domain.Filter) ([]domain.Order, error) {
	return s.orderRepo.FindByFilter(ctx, filter)
}
