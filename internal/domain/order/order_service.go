package order

import (
	"TestTaskJustPay/internal/controller/apperror"
	"context"
	"fmt"
)

type OrderService struct {
	orderRepo OrderRepo
}

func NewOrderService(orderRepo OrderRepo) *OrderService {
	return &OrderService{orderRepo: orderRepo}
}

func (s *OrderService) GetOrderByID(ctx context.Context, id string) (Order, error) {
	return getOrderByID(ctx, s.orderRepo, id)
}

func getOrderByID(ctx context.Context, repo TxOrderRepo, id string) (Order, error) {
	query, _ := NewOrdersQueryBuilder().
		WithIDs(id).
		Build()

	orders, err := repo.GetOrders(ctx, query)
	if err != nil {
		return Order{}, fmt.Errorf("get order: %w", err)
	}
	if len(orders) == 0 {
		return Order{}, apperror.ErrOrderNotFound
	}
	return orders[0], nil
}

func (s *OrderService) GetOrders(ctx context.Context, query OrdersQuery) ([]Order, error) {
	orders, err := s.orderRepo.GetOrders(ctx, &query)
	if err != nil {
		return nil, fmt.Errorf("filter orders: %w", err)
	}
	return orders, nil
}

func (s *OrderService) ProcessEvent(ctx context.Context, event Event) error {
	return s.orderRepo.InTransaction(ctx, func(tx TxOrderRepo) error {
		if event.Status == StatusCreated {
			if err := tx.CreateOrderByEvent(ctx, event); err != nil {
				return fmt.Errorf("create order from event: %w", err)
			}
		} else {
			order, err := getOrderByID(ctx, tx, event.OrderId)
			if err != nil {
				return fmt.Errorf("load order: %w", err)
			}

			if !order.Status.CanBeUpdatedTo(event.Status) {
				return apperror.ErrUnappropriatedStatus
			}

			if err := tx.UpdateOrder(ctx, event); err != nil {
				return fmt.Errorf("update order: %w", err)
			}
		}

		if err := tx.CreateEvent(ctx, event); err != nil {
			return fmt.Errorf("store event: %w", err)
		}
		return nil

	})
}

func (s *OrderService) GetEvents(ctx context.Context, orderID string) ([]EventBase, error) {
	query := NewEventQueryBuilder().
		WithOrderIDs(orderID).
		Build()

	events, err := s.orderRepo.GetEvents(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get events for order %s: %w", orderID, err)
	}
	return events, nil
}
