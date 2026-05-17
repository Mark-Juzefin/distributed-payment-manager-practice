package order

import (
	"context"

	"TestTaskJustPay/services/paymanager/internal/gateway"
)

// OrderRepo is the persistence contract for orders.
type OrderRepo interface {
	CreateOrder(ctx context.Context, update OrderUpdate) error
	GetOrders(ctx context.Context, filter *OrdersQuery) ([]Order, error)
	UpdateOrder(ctx context.Context, update OrderUpdate) error
	UpdateOrderHold(ctx context.Context, request UpdateOrderHoldRequest) error
}

// OrderEvents is the event-sink contract for order domain events.
// TODO: remove in favor of direct eventstore.Store calls.
type OrderEvents interface {
	CreateOrderEvent(ctx context.Context, event NewOrderEvent) (*OrderEvent, error)
	GetOrderEvents(ctx context.Context, query OrderEventQuery) (OrderEventPage, error)
}

// Provider is the minimal interface this domain requires from the payment gateway.
type Provider interface {
	CapturePayment(ctx context.Context, req gateway.CaptureRequest) (gateway.CaptureResult, error)
}
