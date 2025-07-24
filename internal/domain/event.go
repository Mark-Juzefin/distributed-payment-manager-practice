package domain

import (
	"github.com/google/uuid"
	"time"
)

type Event struct {
	EventBase
	Meta map[string]string `json:"meta"`
}

type EventBase struct {
	EventId   string      `json:"event_id"`
	OrderId   string      `json:"order_id"`
	UserId    uuid.UUID   `json:"user_id"`
	Status    OrderStatus `json:"status"`
	UpdatedAt time.Time   `json:"updated_at"`
	CreatedAt time.Time   `json:"created_at"`
}

func NewEventBase(eventId, orderId string, userId uuid.UUID, rawStatus string, createdAt time.Time, updatedAt time.Time) (EventBase, error) {
	status, err := NewOrderStatus(rawStatus)
	if err != nil {
		return EventBase{}, err
	}

	return EventBase{
		EventId:   eventId,
		OrderId:   orderId,
		UserId:    userId,
		Status:    status,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
