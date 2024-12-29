package domain

import (
	"errors"
	"github.com/google/uuid"
	"slices"
	"time"
)

//easyjson:json
type Order struct {
	orderId   uuid.UUID   `json:"order_id"`
	userId    uuid.UUID   `json:"user_id"`
	status    OrderStatus `json:"status"`
	createdAt time.Time   `json:"created_at"`
	updatedAt time.Time   `json:"updated_at"`
}

func (o Order) OrderId() uuid.UUID {
	return o.orderId
}

func (o Order) UserId() uuid.UUID {
	return o.userId
}

func (o Order) Status() OrderStatus {
	return o.status
}

func (o Order) CreatedAt() time.Time {
	return o.createdAt
}

func (o Order) UpdatedAt() time.Time {
	return o.updatedAt
}

func NewOrder(orderId uuid.UUID, userId uuid.UUID, rawStatus string, createdAt time.Time, updatedAt time.Time) (*Order, error) {
	status, err := convertToStatus(rawStatus)
	if err != nil {
		return nil, err
	}

	return &Order{
		orderId:   orderId,
		userId:    userId,
		status:    status,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}, nil
}

type OrderStatus string

var AvailableStatuses = []OrderStatus{"created", "updated", "created", "success"}

func convertToStatus(raw string) (OrderStatus, error) {
	if slices.Contains(AvailableStatuses, OrderStatus(raw)) {
		return OrderStatus(raw), nil
	}
	return "", errors.New("invalid order")
}
