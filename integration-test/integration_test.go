package integration_test

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/app"
	apphttp "TestTaskJustPay/internal/controller/http"
	"TestTaskJustPay/internal/controller/http/handlers"
	"TestTaskJustPay/internal/domain/order"
	order_repo "TestTaskJustPay/internal/repo/order"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestServer(t *testing.T) *httptest.Server {
	cfg, err := config.New()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	l := logger.New(cfg.LogLevel)

	pool, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(cfg.PgPoolMax))
	if err != nil {
		t.Fatalf("Failed to create postgres pool: %v", err)
	}

	// Apply migrations
	err = app.ApplyMigrations(cfg.PgURL, app.MIGRATION_FS)
	if err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}

	// Clean existing data from tables
	_, err = pool.Pool.Exec(context.Background(), "TRUNCATE TABLE order_events, orders CASCADE")
	if err != nil {
		t.Fatalf("Failed to clean database: %v", err)
	}

	repo := order_repo.NewPgOrderRepo(pool)
	iOrderService := order.NewOrderService(repo)
	orderHandler := handlers.NewOrderHandler(iOrderService)
	router := apphttp.NewRouter(orderHandler)

	engine := app.NewGinEngine(l)
	router.SetUp(engine)

	return httptest.NewServer(engine)
}

func TestCreateOrdersFlow(t *testing.T) {
	server := setupTestServer(t)
	defer server.Close()

	// Create multiple orders via webhook
	orders := []map[string]interface{}{
		{
			"event_id":   "evt-1",
			"order_id":   "order-1",
			"user_id":    "11111111-1111-1111-1111-111111111111",
			"status":     "created",
			"updated_at": time.Now().Format(time.RFC3339),
			"created_at": time.Now().Format(time.RFC3339),
			"meta": map[string]string{
				"amount":   "100.50",
				"currency": "USD",
			},
		},
		{
			"event_id":   "evt-2",
			"order_id":   "order-2",
			"user_id":    "22222222-2222-2222-2222-222222222222",
			"status":     "created",
			"updated_at": time.Now().Format(time.RFC3339),
			"created_at": time.Now().Format(time.RFC3339),
			"meta": map[string]string{
				"amount":   "200.75",
				"currency": "EUR",
			},
		},
	}

	// Send webhook events to create orders
	for _, orderData := range orders {
		payload, _ := json.Marshal(orderData)
		resp, err := http.Post(server.URL+"/webhooks/payments/orders", "application/json", bytes.NewBuffer(payload))
		if err != nil {
			t.Fatalf("Failed to send webhook: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 200 or 201, got %d", resp.StatusCode)
		}
	}

	// Get all orders to verify creation
	resp, err := http.Get(server.URL + "/orders")
	if err != nil {
		t.Fatalf("Failed to get orders: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) < 2 {
		t.Errorf("Expected at least 2 orders, got %d", len(result))
	}
}

func TestUpdateOrderFlow(t *testing.T) {
	server := setupTestServer(t)
	defer server.Close()

	orderID := "order-update-test"

	// Create initial order
	initialOrder := map[string]interface{}{
		"event_id":   "evt-create",
		"order_id":   orderID,
		"user_id":    "33333333-3333-3333-3333-333333333333",
		"status":     "created",
		"updated_at": time.Now().Format(time.RFC3339),
		"created_at": time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "150.25",
			"currency": "USD",
		},
	}

	payload, _ := json.Marshal(initialOrder)
	resp, err := http.Post(server.URL+"/webhooks/payments/orders", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 200 or 201, got %d", resp.StatusCode)
	}

	// Verify order was created
	resp, err = http.Get(server.URL + "/orders/" + orderID)
	if err != nil {
		t.Fatalf("Failed to get order: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Resp: %v", string(body))
	}

	// Update the order
	updatedOrder := map[string]interface{}{
		"event_id":   "evt-update",
		"order_id":   orderID,
		"user_id":    "33333333-3333-3333-3333-333333333333",
		"status":     "updated",
		"updated_at": time.Now().Format(time.RFC3339),
		"created_at": time.Now().Add(-time.Hour).Format(time.RFC3339), // Earlier creation time
		"meta": map[string]string{
			"amount":   "150.25",
			"currency": "USD",
		},
	}

	payload, _ = json.Marshal(updatedOrder)
	resp, err = http.Post(server.URL+"/webhooks/payments/orders", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatalf("Failed to update order: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Resp: %v", string(body))
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify order was updated
	resp, err = http.Get(server.URL + "/orders/" + orderID)
	if err != nil {
		t.Fatalf("Failed to get updated order: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Resp: %v", string(body))
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var updatedResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&updatedResult)

	if updatedResult["status"] != "updated" {
		t.Errorf("Expected order status to be 'completed', got %v", updatedResult["status"])
	}
}
