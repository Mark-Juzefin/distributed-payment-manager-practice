package order

import "context"

//go:generate mockgen -source repo.go -destination mock_repo.go -package order

type OrderRepo interface {
	CreateOrder(ctx context.Context, update OrderUpdate) error
	GetOrders(ctx context.Context, filter *OrdersQuery) ([]Order, error)

	UpdateOrder(ctx context.Context, update OrderUpdate) error
	UpdateOrderHold(ctx context.Context, request UpdateOrderHoldRequest) error
}
