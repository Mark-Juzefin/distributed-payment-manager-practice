package order

import (
	"TestTaskJustPay/internal/controller/apperror"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func orderService(t *testing.T) (*OrderService, *MockOrderRepo) {
	t.Helper()

	mockRepo := NewMockOrderRepo(gomock.NewController(t))
	service := NewOrderService(mockRepo)

	return service, mockRepo
}

func TestOrderService_GetOrderByID(t *testing.T) {
	t.Parallel()

	service, mockRepo := orderService(t)

	t.Run("should get order by ID", func(t *testing.T) {
		// given
		ctx := context.Background()
		orderID := "ORDER-123"
		userID := uuid.New()
		expectedOrder := Order{
			OrderId:   orderID,
			UserId:    userID,
			Status:    StatusCreated,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		testCases := []struct {
			name          string
			orderID       string
			mock          func()
			expectedOrder Order
			expectedError error
		}{
			{
				name:    "should return order when found",
				orderID: orderID,
				mock: func() {
					expectedQuery, _ := NewOrdersQueryBuilder().WithIDs(orderID).Build()
					mockRepo.EXPECT().GetOrders(ctx, expectedQuery).Return([]Order{expectedOrder}, nil)
				},
				expectedOrder: expectedOrder,
				expectedError: nil,
			},
			{
				name:    "should return ErrOrderNotFound when order not found",
				orderID: orderID,
				mock: func() {
					expectedQuery, _ := NewOrdersQueryBuilder().WithIDs(orderID).Build()
					mockRepo.EXPECT().GetOrders(ctx, expectedQuery).Return([]Order{}, nil)
				},
				expectedOrder: Order{},
				expectedError: apperror.ErrOrderNotFound,
			},
			{
				name:    "should return error when repository fails",
				orderID: orderID,
				mock: func() {
					expectedQuery, _ := NewOrdersQueryBuilder().WithIDs(orderID).Build()
					mockRepo.EXPECT().GetOrders(ctx, expectedQuery).Return([]Order{}, errors.New("database error"))
				},
				expectedOrder: Order{},
				expectedError: errors.New("get order: database error"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// given
				tc.mock()

				// when
				result, err := service.GetOrderByID(ctx, tc.orderID)

				// then
				assert.EqualValues(t, tc.expectedOrder, result)
				if tc.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
			})
		}
	})
}

func TestOrderService_GetOrders(t *testing.T) {
	t.Parallel()

	service, mockRepo := orderService(t)

	t.Run("should filter orders by query", func(t *testing.T) {
		// given
		ctx := context.Background()
		userID := uuid.New()
		orders := []Order{
			{
				OrderId:   "ORDER-1",
				UserId:    userID,
				Status:    StatusCreated,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				OrderId:   "ORDER-2",
				UserId:    userID,
				Status:    StatusUpdated,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		testCases := []struct {
			name           string
			query          OrdersQuery
			mock           func()
			expectedOrders []Order
			expectedError  error
		}{
			{
				name: "should return filtered orders",
				query: OrdersQuery{
					Statuses: []Status{StatusCreated, StatusUpdated},
				},
				mock: func() {
					mockRepo.EXPECT().GetOrders(ctx, &OrdersQuery{
						Statuses: []Status{StatusCreated, StatusUpdated},
					}).Return(orders, nil)
				},
				expectedOrders: orders,
				expectedError:  nil,
			},
			{
				name: "should return error when repository fails",
				query: OrdersQuery{
					Statuses: []Status{StatusCreated},
				},
				mock: func() {
					mockRepo.EXPECT().GetOrders(ctx, &OrdersQuery{
						Statuses: []Status{StatusCreated},
					}).Return(nil, errors.New("database error"))
				},
				expectedOrders: nil,
				expectedError:  errors.New("filter orders: database error"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// given
				tc.mock()

				// when
				result, err := service.GetOrders(ctx, tc.query)

				// then
				assert.EqualValues(t, tc.expectedOrders, result)
				if tc.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
			})
		}
	})
}

func TestOrderService_ProcessEvent(t *testing.T) {
	t.Parallel()

	service, mockRepo := orderService(t)

	t.Run("should process events correctly", func(t *testing.T) {
		// given
		ctx := context.Background()
		orderID := "ORDER-123"
		userID := uuid.New()
		now := time.Now()

		existingOrder := Order{
			OrderId:   orderID,
			UserId:    userID,
			Status:    StatusCreated,
			CreatedAt: now,
			UpdatedAt: now,
		}

		createEvent := Event{
			EventBase: EventBase{
				EventId:   "EVENT-1",
				OrderId:   orderID,
				Status:    StatusCreated,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Meta: map[string]string{"key": "value"},
		}

		updateEvent := Event{
			EventBase: EventBase{
				EventId:   "EVENT-2",
				OrderId:   orderID,
				Status:    StatusUpdated,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Meta: map[string]string{"key": "value"},
		}

		testCases := []struct {
			name          string
			event         Event
			mock          func(*MockTxOrderRepo)
			expectedError error
		}{
			{
				name:  "should create order and event for StatusCreated",
				event: createEvent,
				mock: func(mockTxRepo *MockTxOrderRepo) {
					mockTxRepo.EXPECT().CreateOrderByEvent(ctx, createEvent).Return(nil)
					mockTxRepo.EXPECT().CreateEvent(ctx, createEvent).Return(nil)
				},
				expectedError: nil,
			},
			{
				name:  "should update order and create event for other statuses",
				event: updateEvent,
				mock: func(mockTxRepo *MockTxOrderRepo) {
					expectedQuery, _ := NewOrdersQueryBuilder().WithIDs(updateEvent.OrderId).Build()
					mockTxRepo.EXPECT().GetOrders(ctx, expectedQuery).Return([]Order{existingOrder}, nil)
					mockTxRepo.EXPECT().UpdateOrder(ctx, updateEvent).Return(nil)
					mockTxRepo.EXPECT().CreateEvent(ctx, updateEvent).Return(nil)
				},
				expectedError: nil,
			},
			{
				name:  "should return error when order creation fails",
				event: createEvent,
				mock: func(mockTxRepo *MockTxOrderRepo) {
					mockTxRepo.EXPECT().CreateOrderByEvent(ctx, createEvent).Return(errors.New("create error"))
				},
				expectedError: errors.New("create order from event: create error"),
			},
			{
				name:  "should return error when order not found for update",
				event: updateEvent,
				mock: func(mockTxRepo *MockTxOrderRepo) {
					expectedQuery, _ := NewOrdersQueryBuilder().WithIDs(updateEvent.OrderId).Build()
					mockTxRepo.EXPECT().GetOrders(ctx, expectedQuery).Return([]Order{}, nil)
				},
				expectedError: errors.New("load order: order not found"),
			},
			{
				name:  "should return error for inappropriate status transition",
				event: updateEvent,
				mock: func(mockTxRepo *MockTxOrderRepo) {
					expectedQuery, _ := NewOrdersQueryBuilder().WithIDs(updateEvent.OrderId).Build()
					mockTxRepo.EXPECT().GetOrders(ctx, expectedQuery).Return([]Order{{
						OrderId: orderID,
						Status:  StatusFailed,
					}}, nil)
				},
				expectedError: apperror.ErrUnappropriatedStatus,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// given
				mockTxRepo := NewMockTxOrderRepo(gomock.NewController(t))
				mockRepo.EXPECT().InTransaction(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(repo TxOrderRepo) error) error {
					return fn(mockTxRepo)
				})
				tc.mock(mockTxRepo)

				// when
				err := service.ProcessEvent(ctx, tc.event)

				// then
				if tc.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
			})
		}
	})
}

func TestOrderService_GetEvents(t *testing.T) {
	t.Parallel()

	service, mockRepo := orderService(t)

	t.Run("should get events for order", func(t *testing.T) {
		// given
		ctx := context.Background()
		orderID := "ORDER-123"
		userID := uuid.New()
		now := time.Now()

		expectedEvents := []EventBase{
			{
				EventId:   "EVENT-1",
				OrderId:   orderID,
				UserId:    userID,
				Status:    StatusCreated,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				EventId:   "EVENT-2",
				OrderId:   orderID,
				UserId:    userID,
				Status:    StatusUpdated,
				CreatedAt: now,
				UpdatedAt: now,
			},
		}

		testCases := []struct {
			name           string
			orderID        string
			mock           func()
			expectedEvents []EventBase
			expectedError  error
		}{
			{
				name:    "should return events for order",
				orderID: orderID,
				mock: func() {
					expectedQuery := NewEventQueryBuilder().WithOrderIDs(orderID).Build()
					mockRepo.EXPECT().GetEvents(ctx, expectedQuery).Return(expectedEvents, nil)
				},
				expectedEvents: expectedEvents,
				expectedError:  nil,
			},
			{
				name:    "should return error when repository fails",
				orderID: orderID,
				mock: func() {
					expectedQuery := NewEventQueryBuilder().WithOrderIDs(orderID).Build()
					mockRepo.EXPECT().GetEvents(ctx, expectedQuery).Return(nil, errors.New("database error"))
				},
				expectedEvents: nil,
				expectedError:  errors.New("get events for order ORDER-123: database error"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// given
				tc.mock()

				// when
				result, err := service.GetEvents(ctx, tc.orderID)

				// then
				assert.EqualValues(t, tc.expectedEvents, result)
				if tc.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
			})
		}
	})
}
