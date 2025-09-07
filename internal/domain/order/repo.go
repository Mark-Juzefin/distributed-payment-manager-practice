package order

import "context"

//go:generate mockgen -source repo.go -destination mock_repo.go -package order

type OrderRepo interface {
	TxOrderRepo
	InTransaction(ctx context.Context, fn func(repo TxOrderRepo) error) error
}

type TxOrderRepo interface {
	CreateOrder(ctx context.Context, payload PaymentWebhook) error
	GetOrders(ctx context.Context, filter *OrdersQuery) ([]Order, error)

	UpdateOrder(ctx context.Context, event PaymentWebhook) error
	UpdateOrderHold(ctx context.Context, request UpdateOrderHoldRequest) error
}
