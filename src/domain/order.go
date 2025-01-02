package domain

import (
	"errors"
	"fmt"
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
	status, err := NewOrderStatus(rawStatus)
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

const (
	StatusCreated OrderStatus = "created"
	StatusUpdated OrderStatus = "updated"
	StatusFailed  OrderStatus = "failed"
	StatusSuccess OrderStatus = "success"
)

var AvailableStatuses = []OrderStatus{StatusCreated, StatusUpdated, StatusFailed, StatusSuccess}

func (s OrderStatus) CanBeUpdatedTo(newStatus OrderStatus) bool {
	switch s {
	case StatusCreated:
		return slices.Contains([]OrderStatus{StatusUpdated, StatusFailed, StatusSuccess}, newStatus)
	case StatusUpdated:
		return slices.Contains([]OrderStatus{StatusFailed, StatusSuccess}, newStatus)
	case StatusFailed, StatusSuccess:
		return false
	default:
		return false
	}
}
func NewOrderStatus(raw string) (OrderStatus, error) {
	if slices.Contains(AvailableStatuses, OrderStatus(raw)) {
		return OrderStatus(raw), nil
	}
	return "", errors.New("invalid order status")
}

type Filter struct {
	Status    []OrderStatus
	UserID    uuid.UUID
	Limit     int
	Offset    int
	SortBy    SortBy
	SortOrder SortOrder
}

type SortBy string

// TODO: use it
var SortByCreatedAt SortBy = "created_at"
var SortByUpdatedAt SortBy = "updated_at"

type SortOrder string

// TODO: use it
var SortOrderDesc SortOrder = "desc"
var SortOrderAsc SortOrder = "asc"

func NewFilter(status []OrderStatus, userID uuid.UUID, limit, offset int, sortBy, sortOrder string) Filter {
	return Filter{
		Status:    status,
		UserID:    userID,
		Limit:     limit,
		Offset:    offset,
		SortBy:    SortBy(sortBy),
		SortOrder: SortOrder(sortOrder),
	}
}
