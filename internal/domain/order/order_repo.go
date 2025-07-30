package order

import "context"

//go:generate mockgen -source order_repo.go -destination mock_order_repo.go -package order

type OrderRepo interface {
	TxOrderRepo
	InTransaction(ctx context.Context, fn func(repo TxOrderRepo) error) error
}

type TxOrderRepo interface {
	GetOrders(ctx context.Context, filter *OrdersQuery) ([]Order, error)
	GetEvents(ctx context.Context, query *EventQuery) ([]EventBase, error)

	UpdateOrder(ctx context.Context, event Event) error
	CreateEvent(ctx context.Context, event Event) error
	CreateOrderByEvent(ctx context.Context, event Event) error
}
