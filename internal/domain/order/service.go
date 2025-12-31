package order

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/gateway"
	"TestTaskJustPay/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OrderService struct {
	orderRepo OrderRepo
	provider  gateway.Provider
	eventSink EventSink
	logger    logger.Interface
}

func NewOrderService(orderRepo OrderRepo, provider gateway.Provider, eventSink EventSink, l logger.Interface) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
		provider:  provider,
		eventSink: eventSink,
		logger:    l,
	}
}

func (s *OrderService) GetOrderByID(ctx context.Context, id string) (Order, error) {
	return getOrderByID(ctx, s.orderRepo, id)
}

func getOrderByID(ctx context.Context, repo TxOrderRepo, id string) (Order, error) {
	query, _ := NewOrdersQueryBuilder().
		WithIDs(id).
		Build()

	orders, err := repo.GetOrders(ctx, query)
	if err != nil {
		return Order{}, fmt.Errorf("get order: %w", err)
	}
	if len(orders) == 0 {
		return Order{}, apperror.ErrOrderNotFound
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

func (s *OrderService) ProcessPaymentWebhook(ctx context.Context, event PaymentWebhook) error {
	err := s.orderRepo.InTransaction(ctx, func(tx TxOrderRepo) error {
		if event.Status == StatusCreated {
			if err := tx.CreateOrder(ctx, event); err != nil {
				return fmt.Errorf("create order from event: %w", err)
			}
		} else {
			order, err := getOrderByID(ctx, tx, event.OrderId)
			if err != nil {
				return fmt.Errorf("load order: %w", err)
			}

			if !order.Status.CanBeUpdatedTo(event.Status) {
				return apperror.ErrUnappropriatedStatus
			}

			if err := tx.UpdateOrder(ctx, event); err != nil {
				return fmt.Errorf("update order: %w", err)
			}
		}

		return nil

	})
	if err != nil {
		return err
	}

	if err := s.createWebhookReceivedEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create webhook event: %w", err)
	}

	return nil
}

func (s *OrderService) GetEvents(ctx context.Context, query OrderEventQuery) (OrderEventPage, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}

	eventPage, err := s.eventSink.GetOrderEvents(ctx, query)
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
	err := s.orderRepo.InTransaction(ctx, func(tx TxOrderRepo) error {
		order, err := getOrderByID(ctx, tx, orderID)
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

		err = tx.UpdateOrderHold(ctx, UpdateOrderHoldRequest{
			OrderID: order.OrderId,
			OnHold:  onHold,
			Reason:  reason,
		})
		if err != nil {
			return fmt.Errorf("update order hold status: %w", err)
		}

		updatedOrder, err := getOrderByID(ctx, tx, order.OrderId)
		if err != nil {
			return fmt.Errorf("get updated order: %w", err)
		}

		response = &HoldResponse{
			OrderID:   updatedOrder.OrderId,
			OnHold:    updatedOrder.OnHold,
			Reason:    updatedOrder.HoldReason,
			UpdatedAt: updatedOrder.UpdatedAt,
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
		s.logger.Error("Failed to create hold event: %v", err)
	}

	return response, nil
}

func (s *OrderService) CapturePayment(ctx context.Context, orderID string, request CaptureRequest) (*CaptureResponse, error) {
	var response *CaptureResponse
	err := s.orderRepo.InTransaction(ctx, func(tx TxOrderRepo) error {
		order, err := getOrderByID(ctx, tx, orderID)
		if err != nil {
			return fmt.Errorf("load order: %w", err)
		}

		// Check if order is on hold
		if order.OnHold {
			return apperror.ErrOrderOnHold
		}

		// Check if order is already in final status (success/failed)
		if order.Status == StatusSuccess || order.Status == StatusFailed {
			return apperror.ErrOrderInFinalStatus
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
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create capture events after successful transaction
	if err := s.createCaptureRequestedEvent(ctx, orderID, request); err != nil {
		s.logger.Error("Failed to create capture requested event: %v", err)
	}

	eventKind := OrderEventCaptureCompleted
	if response.Status == "failed" {
		eventKind = OrderEventCaptureFailed
	}
	if err := s.createCaptureResultEvent(ctx, orderID, eventKind, *response); err != nil {
		s.logger.Error("Failed to create capture result event: %v", err)
	}

	return response, nil
}

// Event creation helper methods
func (s *OrderService) createWebhookReceivedEvent(ctx context.Context, webhook PaymentWebhook) error {
	payload, _ := json.Marshal(webhook)

	orderEvent := NewOrderEvent{
		OrderID:         webhook.OrderId,
		Kind:            OrderEventWebhookReceived,
		ProviderEventID: webhook.ProviderEventID,
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	_, err := s.eventSink.CreateOrderEvent(ctx, orderEvent)
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

	_, err := s.eventSink.CreateOrderEvent(ctx, orderEvent)
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

	_, err := s.eventSink.CreateOrderEvent(ctx, orderEvent)
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

	_, err := s.eventSink.CreateOrderEvent(ctx, orderEvent)
	return err
}
