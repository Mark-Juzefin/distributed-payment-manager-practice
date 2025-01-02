package domain

import (
	"github.com/google/uuid"
	"time"
)

type Event struct {
	EventId   string            `json:"event_id"`
	OrderId   string            `json:"order_id"`
	UserId    uuid.UUID         `json:"user_id"`
	Status    OrderStatus       `json:"status"`
	UpdatedAt time.Time         `json:"updated_at"`
	CreatedAt time.Time         `json:"created_at"`
	Meta      map[string]string `json:"meta"`
}
