package domain

import (
	"errors"
	"github.com/google/uuid"
	"slices"
	"time"
)

type Order struct {
	OrderId   string      `json:"order_id"`
	UserId    uuid.UUID   `json:"user_id"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

func NewOrder(orderId string, userId uuid.UUID, rawStatus string, createdAt time.Time, updatedAt time.Time) (Order, error) {
	status, err := convertToStatus(rawStatus)
	if err != nil {
		return Order{}, err
	}

	return Order{
		OrderId:   orderId,
		UserId:    userId,
		Status:    status,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
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
