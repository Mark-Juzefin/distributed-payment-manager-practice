package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/domain/events"
	"TestTaskJustPay/services/paymanager/domain/gateway"

	"github.com/google/uuid"
)

type OrderService struct {
	transactor   postgres.Transactor
	txOrderRepo  func(tx postgres.Executor) OrderRepo
	txEventStore func(tx postgres.Executor) events.Store
	orderRepo    OrderRepo // for reads (GetOrders, GetOrderByID)
	provider     gateway.Provider
	orderEvents  OrderEvents
}

func NewOrderService(
	transactor postgres.Transactor,
	txOrderRepo func(tx postgres.Executor) OrderRepo,
	txEventStore func(tx postgres.Executor) events.Store,
	orderRepo OrderRepo,
	provider gateway.Provider,
	orderEvents OrderEvents,
) *OrderService {
	return &OrderService{
		transactor:   transactor,
		txOrderRepo:  txOrderRepo,
		txEventStore: txEventStore,
		orderRepo:    orderRepo,
		provider:     provider,
		orderEvents:  orderEvents,
	}
}

func (s *OrderService) GetOrderByID(ctx context.Context, id string) (Order, error) {
	return getOrderByID(ctx, s.orderRepo, id)
}

func getOrderByID(ctx context.Context, repo OrderRepo, id string) (Order, error) {
	query, _ := NewOrdersQueryBuilder().
		WithIDs(id).
		Build()

	orders, err := repo.GetOrders(ctx, query)
	if err != nil {
		return Order{}, fmt.Errorf("get order: %w", err)
	}
	if len(orders) == 0 {
		return Order{}, ErrNotFound
	}
	return orders[0], nil
}

func (s *OrderService) GetOrders(ctx context.Context, query OrdersQuery) ([]Order, error) {
	orders, err := s.orderRepo.GetOrders(ctx, &query)
	if err != nil {
		return nil, fmt.Errorf("filter orders: %w", err)
	}
	return orders, nil
}

func (s *OrderService) ProcessOrderUpdate(ctx context.Context, update OrderUpdate) error {
	err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txOrderRepo(tx)
		txEvents := s.txEventStore(tx)

		if update.Status == StatusCreated {
			if err := txRepo.CreateOrder(ctx, update); err != nil {
				return fmt.Errorf("create order from event: %w", err)
			}
		} else {
			order, err := getOrderByID(ctx, txRepo, update.OrderId)
			if err != nil {
				return fmt.Errorf("load order: %w", err)
			}

			if !order.Status.CanBeUpdatedTo(update.Status) {
				return ErrInvalidStatus
			}

			if err := txRepo.UpdateOrder(ctx, update); err != nil {
				return fmt.Errorf("update order: %w", err)
			}
		}

		// Write unified event (inside transaction)
		payload, _ := json.Marshal(update)
		if err := s.writeEvent(ctx, txEvents, events.AggregateOrder, update.OrderId,
			string(OrderEventWebhookReceived), update.ProviderEventID, payload); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if err := s.createWebhookReceivedEvent(ctx, update); err != nil {
		return fmt.Errorf("failed to create webhook event: %w", err)
	}

	return nil
}

func (s *OrderService) GetEvents(ctx context.Context, query OrderEventQuery) (OrderEventPage, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}

	eventPage, err := s.orderEvents.GetOrderEvents(ctx, query)
	if err != nil {
		return OrderEventPage{}, fmt.Errorf("get order events: %w", err)
	}
	return eventPage, nil
}

func (s *OrderService) UpdateOrderHold(ctx context.Context, orderID string, request HoldRequest) (*HoldResponse, error) {
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("invalid hold request: %w", err)
	}

	var response *HoldResponse
	err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txOrderRepo(tx)
		txEvents := s.txEventStore(tx)

		order, err := getOrderByID(ctx, txRepo, orderID)
		if err != nil {
			return fmt.Errorf("load order: %w", err)
		}

		var onHold bool
		var reason *string

		switch request.Action {
		case HoldActionSet:
			onHold = true
			reasonStr := string(*request.Reason)
			reason = &reasonStr
		case HoldActionClear:
			onHold = false
			reason = nil
		}

		err = txRepo.UpdateOrderHold(ctx, UpdateOrderHoldRequest{
			OrderID: order.OrderId,
			OnHold:  onHold,
			Reason:  reason,
		})
		if err != nil {
			return fmt.Errorf("update order hold status: %w", err)
		}

		updatedOrder, err := getOrderByID(ctx, txRepo, order.OrderId)
		if err != nil {
			return fmt.Errorf("get updated order: %w", err)
		}

		response = &HoldResponse{
			OrderID:   updatedOrder.OrderId,
			OnHold:    updatedOrder.OnHold,
			Reason:    updatedOrder.HoldReason,
			UpdatedAt: updatedOrder.UpdatedAt,
		}

		// Write unified event (inside transaction)
		eventKind := OrderEventHoldSet
		if request.Action == HoldActionClear {
			eventKind = OrderEventHoldCleared
		}
		payload, _ := json.Marshal(request)
		if err := s.writeEvent(ctx, txEvents, events.AggregateOrder, orderID,
			string(eventKind), uuid.New().String(), payload); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create hold event after successful transaction
	eventKind := OrderEventHoldSet
	if request.Action == HoldActionClear {
		eventKind = OrderEventHoldCleared
	}
	if err := s.createHoldEvent(ctx, orderID, eventKind, request); err != nil {
		slog.ErrorContext(ctx, "Failed to create hold event", slog.Any("error", err))
	}

	return response, nil
}

func (s *OrderService) CapturePayment(ctx context.Context, orderID string, request CaptureRequest) (*CaptureResponse, error) {
	var response *CaptureResponse
	err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txOrderRepo(tx)
		txEvents := s.txEventStore(tx)

		order, err := getOrderByID(ctx, txRepo, orderID)
		if err != nil {
			return fmt.Errorf("load order: %w", err)
		}

		// Check if order is on hold
		if order.OnHold {
			return ErrOnHold
		}

		// Check if order is already in final status (success/failed)
		if order.Status == StatusSuccess || order.Status == StatusFailed {
			return ErrInFinalStatus
		}

		// Call provider to capture payment
		captureReq := gateway.CaptureRequest{
			OrderID:        order.OrderId,
			Amount:         request.Amount,
			Currency:       request.Currency,
			IdempotencyKey: request.IdempotencyKey,
		}

		result, err := s.provider.CapturePayment(ctx, captureReq)
		if err != nil {
			return fmt.Errorf("provider capture failed: %w", err)
		}

		response = &CaptureResponse{
			OrderID:      order.OrderId,
			Amount:       request.Amount,
			Currency:     request.Currency,
			Status:       string(result.Status),
			ProviderTxID: result.ProviderTxID,
			CapturedAt:   time.Now(),
		}

		// Write unified events (inside transaction)
		reqPayload, _ := json.Marshal(request)
		if err := s.writeEvent(ctx, txEvents, events.AggregateOrder, orderID,
			string(OrderEventCaptureRequested), request.IdempotencyKey, reqPayload); err != nil {
			return err
		}

		resultKind := OrderEventCaptureCompleted
		if response.Status == "failed" {
			resultKind = OrderEventCaptureFailed
		}
		resPayload, _ := json.Marshal(response)
		if err := s.writeEvent(ctx, txEvents, events.AggregateOrder, orderID,
			string(resultKind), response.ProviderTxID, resPayload); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create capture events after successful transaction
	if err := s.createCaptureRequestedEvent(ctx, orderID, request); err != nil {
		slog.ErrorContext(ctx, "Failed to create capture requested event", slog.Any("error", err))
	}

	eventKind := OrderEventCaptureCompleted
	if response.Status == "failed" {
		eventKind = OrderEventCaptureFailed
	}
	if err := s.createCaptureResultEvent(ctx, orderID, eventKind, *response); err != nil {
		slog.ErrorContext(ctx, "Failed to create capture result event", slog.Any("error", err))
	}

	return response, nil
}

// Event creation helper methods
func (s *OrderService) createWebhookReceivedEvent(ctx context.Context, webhook OrderUpdate) error {
	payload, _ := json.Marshal(webhook)

	orderEvent := NewOrderEvent{
		OrderID:         webhook.OrderId,
		Kind:            OrderEventWebhookReceived,
		ProviderEventID: webhook.ProviderEventID,
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	_, err := s.orderEvents.CreateOrderEvent(ctx, orderEvent)
	return err
}

func (s *OrderService) createHoldEvent(ctx context.Context, orderID string, kind OrderEventKind, request HoldRequest) error {
	payload, _ := json.Marshal(request)

	orderEvent := NewOrderEvent{
		OrderID:         orderID,
		Kind:            kind,
		ProviderEventID: uuid.New().String(), // Generate unique ID for internal hold operations
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	_, err := s.orderEvents.CreateOrderEvent(ctx, orderEvent)
	return err
}

func (s *OrderService) createCaptureRequestedEvent(ctx context.Context, orderID string, request CaptureRequest) error {
	payload, _ := json.Marshal(request)

	orderEvent := NewOrderEvent{
		OrderID:         orderID,
		Kind:            OrderEventCaptureRequested,
		ProviderEventID: request.IdempotencyKey, // Use idempotency key as provider event ID
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	_, err := s.orderEvents.CreateOrderEvent(ctx, orderEvent)
	return err
}

func (s *OrderService) createCaptureResultEvent(ctx context.Context, orderID string, kind OrderEventKind, response CaptureResponse) error {
	payload, _ := json.Marshal(response)

	orderEvent := NewOrderEvent{
		OrderID:         orderID,
		Kind:            kind,
		ProviderEventID: response.ProviderTxID,
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	_, err := s.orderEvents.CreateOrderEvent(ctx, orderEvent)
	return err
}

// writeEvent writes to the unified events table. Duplicate events are silently ignored (idempotent).
func (s *OrderService) writeEvent(ctx context.Context, store events.Store, aggregateType events.AggregateType, aggregateID, eventType, idempotencyKey string, payload json.RawMessage) error {
	_, err := store.CreateEvent(ctx, events.NewEvent{
		AggregateType:  aggregateType,
		AggregateID:    aggregateID,
		EventType:      eventType,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
		CreatedAt:      time.Now(),
	})
	if errors.Is(err, events.ErrEventAlreadyStored) {
		return nil // idempotent — duplicate is a no-op
	}
	if err != nil {
		return fmt.Errorf("write unified event: %w", err)
	}
	return nil
}
