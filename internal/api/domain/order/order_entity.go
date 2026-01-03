package order

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

type Order struct {
	OrderId    string    `json:"order_id"`
	UserId     uuid.UUID `json:"user_id"`
	Status     Status    `json:"status"`
	OnHold     bool      `json:"on_hold"`
	HoldReason *string   `json:"hold_reason,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Status string

const (
	StatusCreated Status = "created"
	StatusUpdated Status = "updated"
	StatusFailed  Status = "failed"
	StatusSuccess Status = "success"
)

var AvailableStatuses = []Status{StatusCreated, StatusUpdated, StatusFailed, StatusSuccess}

func (s Status) CanBeUpdatedTo(newStatus Status) bool {
	switch s {
	case StatusCreated:
		return slices.Contains([]Status{StatusUpdated, StatusFailed, StatusSuccess}, newStatus)
	case StatusUpdated:
		return slices.Contains([]Status{StatusUpdated, StatusFailed, StatusSuccess}, newStatus)
	case StatusFailed, StatusSuccess:
		return false
	default:
		return false
	}
}
func NewStatus(raw string) (Status, error) {
	if slices.Contains(AvailableStatuses, Status(raw)) {
		return Status(raw), nil
	}
	return "", errors.New("invalid order status")
}

type Pagination struct {
	PageSize int

	PageNumber int
}

type OrdersQuery struct {
	IDs        []string
	UserIDs    []string
	Statuses   []Status
	Pagination *Pagination
	SortBy     *string
	SortOrder  *string
}

func (o *OrdersQuery) Validate() error {
	if o.SortBy != nil && *o.SortBy != "created_at" && *o.SortBy != "updated_at" {
		return fmt.Errorf("invalid sort by: %s", *o.SortBy)
	}
	if o.SortOrder != nil && *o.SortOrder != "asc" && *o.SortOrder != "desc" {
		return fmt.Errorf("invalid sort order: %s", *o.SortOrder)
	}
	return nil
}

type OrdersQueryBuilder struct {
	query *OrdersQuery
}

func NewOrdersQueryBuilder() *OrdersQueryBuilder {
	return &OrdersQueryBuilder{
		query: &OrdersQuery{},
	}
}

func (b *OrdersQueryBuilder) Build() (*OrdersQuery, error) {
	if err := b.query.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidQuery, err.Error())
	}
	return b.query, nil
}

func (b *OrdersQueryBuilder) WithIDs(ids ...string) *OrdersQueryBuilder {
	b.query.IDs = ids
	return b
}

func (b *OrdersQueryBuilder) WithUserIDs(userIDs ...string) *OrdersQueryBuilder {
	b.query.UserIDs = userIDs
	return b
}

func (b *OrdersQueryBuilder) WithStatuses(statuses ...Status) *OrdersQueryBuilder {
	b.query.Statuses = statuses
	return b
}

// todo: bad design - create struct
func (b *OrdersQueryBuilder) WithSort(sortBy, sortOrder string) *OrdersQueryBuilder {
	b.query.SortBy = &sortBy
	b.query.SortOrder = &sortOrder
	return b
}

func (b *OrdersQueryBuilder) WithPagination(pagination Pagination) *OrdersQueryBuilder {
	b.query.Pagination = &pagination
	return b
}

type HoldAction string

const (
	HoldActionSet   HoldAction = "set"
	HoldActionClear HoldAction = "clear"
)

type HoldReason string

const (
	HoldReasonManualReview HoldReason = "manual_review"
	HoldReasonRisk         HoldReason = "risk"
)

type HoldRequest struct {
	Action HoldAction  `json:"action" binding:"required,oneof=set clear"`
	Reason *HoldReason `json:"reason,omitempty" binding:"omitempty,oneof=manual_review risk"`
}

func (hr *HoldRequest) Validate() error {
	if hr.Action == HoldActionSet && hr.Reason == nil {
		return errors.New("reason is required when action is 'set'")
	}
	return nil
}

type HoldResponse struct {
	OrderID   string    `json:"order_id"`
	OnHold    bool      `json:"on_hold"`
	Reason    *string   `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateOrderHoldRequest struct {
	OrderID string
	OnHold  bool
	Reason  *string
}

type CaptureRequest struct {
	Amount         float64 `json:"amount" binding:"required,min=1"`
	Currency       string  `json:"currency" binding:"required,len=3"`
	IdempotencyKey string  `json:"idempotency_key" binding:"required,min=1,max=255"`
}

type CaptureResponse struct {
	OrderID      string    `json:"order_id"`
	Amount       float64   `json:"amount"`
	Currency     string    `json:"currency"`
	Status       string    `json:"status"`
	ProviderTxID string    `json:"provider_tx_id"`
	CapturedAt   time.Time `json:"captured_at"`
}
