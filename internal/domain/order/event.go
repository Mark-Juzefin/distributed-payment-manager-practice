package order

import (
	"github.com/google/uuid"
	"time"
)

type Event struct {
	EventBase
	Meta map[string]string `json:"meta"`
}

type EventBase struct {
	EventId   string    `json:"event_id"`
	OrderId   string    `json:"order_id"`
	UserId    uuid.UUID `json:"user_id"`
	Status    Status    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

func NewEventBase(eventId, orderId string, userId uuid.UUID, rawStatus string, createdAt time.Time, updatedAt time.Time) (EventBase, error) {
	status, err := NewStatus(rawStatus)
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

type EventQuery struct {
	OrderIDs []string
	UserIDs  []string
	Statuses []Status
}

type EventQueryBuilder struct {
	query *EventQuery
}

func NewEventQueryBuilder() *EventQueryBuilder {
	return &EventQueryBuilder{
		query: &EventQuery{},
	}
}

func (b *EventQueryBuilder) WithOrderIDs(orderIDs ...string) *EventQueryBuilder {
	b.query.OrderIDs = orderIDs
	return b
}

func (b *EventQueryBuilder) WithUserIDs(userIDs ...string) *EventQueryBuilder {
	b.query.UserIDs = userIDs
	return b
}

func (b *EventQueryBuilder) WithStatuses(statuses ...Status) *EventQueryBuilder {
	b.query.Statuses = statuses
	return b
}

func (b *EventQueryBuilder) Build() *EventQuery {
	return b.query
}
