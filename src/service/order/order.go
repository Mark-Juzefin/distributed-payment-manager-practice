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

//func (s *orderService) GetAll(ctx context.Context) ([]domain.Order, error) {
//	return s.orderRepo.FindAll(ctx)
//}
