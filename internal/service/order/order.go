package order

import (
	domain2 "TestTaskJustPay/internal/domain"
	"TestTaskJustPay/internal/service"
	"context"
)

type orderService struct {
	orderRepo Repo
}

func NewOrderService(orderRepo Repo) service.IOrderService {
	return &orderService{orderRepo}
}

func (s *orderService) Get(ctx context.Context, id string) (domain2.Order, error) {
	return s.orderRepo.FindById(ctx, id)
}

func (s *orderService) Filter(ctx context.Context, filter domain2.Filter) ([]domain2.Order, error) {
	found, err := s.orderRepo.FindByFilter(ctx, filter)
	if found == nil {
		found = []domain2.Order{}
	}
	return found, err
}

func (s *orderService) ProcessEvent(ctx context.Context, event domain2.Event) error {
	return s.orderRepo.UpdateOrderAndSaveEvent(ctx, event)
}

func (s *orderService) GetEvents(ctx context.Context, orderID string) ([]domain2.EventBase, error) {
	found, err := s.orderRepo.GetEventsByOrderId(ctx, orderID)
	if found == nil {
		found = []domain2.EventBase{}
	}
	return found, err
}
