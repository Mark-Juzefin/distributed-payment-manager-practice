package order

import "context"

type TxOrderRepo interface {
	GetOrders(ctx context.Context, filter *OrdersQuery) ([]Order, error)
	GetEvents(ctx context.Context, query *EventQuery) ([]EventBase, error)

	UpdateOrder(ctx context.Context, event Event) error
	CreateEvent(ctx context.Context, event Event) error
	CreateOrderByEvent(ctx context.Context, event Event) error
}

type OrderRepo interface {
	TxOrderRepo
	InTransaction(ctx context.Context, fn func(repo TxOrderRepo) error) error
}
