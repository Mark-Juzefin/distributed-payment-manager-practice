package order

import "time"

type PaymentWebhook struct {
	ProviderEventID string            `json:"provider_event_id"`
	OrderId         string            `json:"order_id"`
	UserId          string            `json:"user_id"`
	Status          Status            `json:"status"`
	UpdatedAt       time.Time         `json:"updated_at"`
	CreatedAt       time.Time         `json:"created_at"`
	Meta            map[string]string `json:"meta"`
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

func (b *EventQueryBuilder) WithStatuses(statuses ...Status) *EventQueryBuilder {
	b.query.Statuses = statuses
	return b
}

func (b *EventQueryBuilder) Build() *EventQuery {
	return b.query
}
