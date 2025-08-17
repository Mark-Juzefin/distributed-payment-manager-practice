//go:build integration
// +build integration

package integration_test

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/app"
	apphttp "TestTaskJustPay/internal/controller/http"
	"TestTaskJustPay/internal/controller/http/handlers"
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	dispute_repo "TestTaskJustPay/internal/repo/dispute"
	order_repo "TestTaskJustPay/internal/repo/order"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// todo: refactor
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
	_, err = pool.Pool.Exec(context.Background(), "TRUNCATE TABLE dispute_events, disputes, order_events, orders CASCADE")
	if err != nil {
		t.Fatalf("Failed to clean database: %v", err)
	}

	orderRepo := order_repo.NewPgOrderRepo(pool)
	disputeRepo := dispute_repo.NewPgDisputeRepo(pool)

	orderService := order.NewOrderService(orderRepo)
	disputeService := dispute.NewDisputeService(disputeRepo)

	orderHandler := handlers.NewOrderHandler(orderService)
	chargebackHandler := handlers.NewChargebackHandler(disputeService)
	disputeHandler := handlers.NewDisputeHandler(disputeService)

	router := apphttp.NewRouter(orderHandler, chargebackHandler, disputeHandler)

	engine := app.NewGinEngine(l)
	router.SetUp(engine)

	return httptest.NewServer(engine)
}

func TestChargebackFlow(t *testing.T) {
	server := setupTestServer(t)
	defer server.Close()

	orderID := "order-chargeback-1"

	// First create an order (required for foreign key constraint)
	createOrder := map[string]interface{}{
		"event_id":   "evt-order-1",
		"order_id":   orderID,
		"user_id":    "44444444-4444-4444-4444-444444444444",
		"status":     "created",
		"updated_at": time.Now().Format(time.RFC3339),
		"created_at": time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "100.50",
			"currency": "USD",
		},
	}

	orderPayload, _ := json.Marshal(createOrder)
	createOrderResp, err := http.Post(server.URL+"/webhooks/payments/orders", "application/json", bytes.NewBuffer(orderPayload))
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}
	createOrderResp.Body.Close()

	if createOrderResp.StatusCode != http.StatusOK && createOrderResp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 200 or 201 for order creation, got %d", createOrderResp.StatusCode)
	}

	// Now create the chargeback to trigger dispute creation
	openChargeback := map[string]interface{}{
		"provider_event_id": "evt-1",
		"order_id":          orderID,
		"status":            "opened",
		"reason":            "fraud",
		"amount":            100.50,
		"currency":          "USD",
		"occurred_at":       time.Now().Format(time.RFC3339),
		"meta":              map[string]string{},
	}

	// send event to create dispute
	openChargebackPayload, _ := json.Marshal(openChargeback)
	openChargebackResp, err := http.Post(server.URL+"/webhooks/payments/chargebacks", "application/json", bytes.NewBuffer(openChargebackPayload))
	if err != nil {
		t.Fatalf("Failed to send chargeback webhook: %v", err)
	}
	openChargebackResp.Body.Close()

	if openChargebackResp.StatusCode != http.StatusOK && openChargebackResp.StatusCode != http.StatusCreated && openChargebackResp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 200 or 201, got %d", openChargebackResp.StatusCode)
	}

	// get all disputes
	disputes := getAllDisputes(t, server)
	if len(disputes) == 0 {
		t.Errorf("Expected at least 1 dispute, got %d", len(disputes))
	}

	// find one by order id
	foundDispute := findDisputeByOrderID(disputes, orderID)
	if foundDispute == nil {
		t.Fatalf("Could not find dispute for order_id: %s", orderID)
	}

	if foundDispute["status"] != "open" {
		t.Errorf("Expected dispute status to be 'open', got %v", foundDispute["status"])
	}
	if foundDispute["reason"] != "fraud" {
		t.Errorf("Expected dispute reason to be 'fraud', got %v", foundDispute["reason"])
	}

	// send event to update dispute (close it as won)
	closeChargeback := map[string]interface{}{
		"provider_event_id": "evt-2",
		"order_id":          orderID,
		"transaction_id":    "txn-chargeback-1",
		"status":            "closed",
		"reason":            "fraud",
		"amount":            100.50,
		"currency":          "USD",
		"occurred_at":       time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"resolution": "won",
		},
	}

	closeChargebackPayload, _ := json.Marshal(closeChargeback)
	closeChargebackResp, err := http.Post(server.URL+"/webhooks/payments/chargebacks", "application/json", bytes.NewBuffer(closeChargebackPayload))
	if err != nil {
		t.Fatalf("Failed to send close chargeback webhook: %v", err)
	}
	closeChargebackResp.Body.Close()

	if closeChargebackResp.StatusCode != http.StatusOK && closeChargebackResp.StatusCode != http.StatusCreated && closeChargebackResp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 200 or 201, got %d", closeChargebackResp.StatusCode)
	}

	// get all disputes again
	disputesAfterUpdate := getAllDisputes(t, server)

	// find updated dispute by order id
	updatedDispute := findDisputeByOrderID(disputesAfterUpdate, orderID)
	if updatedDispute == nil {
		t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
	}

	if updatedDispute["status"] != "won" {
		t.Errorf("Expected updated dispute status to be 'won', got %v", updatedDispute["status"])
	}

	// verify closed_at timestamp is set
	if updatedDispute["closed_at"] == nil {
		t.Errorf("Expected closed_at to be set for closed dispute")
	}
}

// Utility functions
func getAllDisputes(t *testing.T, server *httptest.Server) []map[string]interface{} {
	resp, err := http.Get(server.URL + "/disputes")
	if err != nil {
		t.Fatalf("Failed to get disputes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var disputes []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&disputes)
	return disputes
}

func findDisputeByOrderID(disputes []map[string]interface{}, orderID string) map[string]interface{} {
	for _, dispute := range disputes {
		if dispute["order_id"] == orderID {
			return dispute
		}
	}
	return nil
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

func TestEvidenceAdditionFlow(t *testing.T) {
	server := setupTestServer(t)
	defer server.Close()

	orderID := "order-evidence-test"

	// First create an order (required for foreign key constraint)
	createOrder := map[string]interface{}{
		"event_id":   "evt-order-evidence",
		"order_id":   orderID,
		"user_id":    "55555555-5555-5555-5555-555555555555",
		"status":     "created",
		"updated_at": time.Now().Format(time.RFC3339),
		"created_at": time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "250.75",
			"currency": "USD",
		},
	}

	orderPayload, _ := json.Marshal(createOrder)
	createOrderResp, err := http.Post(server.URL+"/webhooks/payments/orders", "application/json", bytes.NewBuffer(orderPayload))
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}
	createOrderResp.Body.Close()

	if createOrderResp.StatusCode != http.StatusOK && createOrderResp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 200 or 201 for order creation, got %d", createOrderResp.StatusCode)
	}

	// Create a chargeback to trigger dispute creation
	openChargeback := map[string]interface{}{
		"provider_event_id": "evt-evidence-1",
		"order_id":          orderID,
		"status":            "opened",
		"reason":            "unauthorized",
		"amount":            250.75,
		"currency":          "USD",
		"occurred_at":       time.Now().Format(time.RFC3339),
		"meta":              map[string]string{},
	}

	openChargebackPayload, _ := json.Marshal(openChargeback)
	openChargebackResp, err := http.Post(server.URL+"/webhooks/payments/chargebacks", "application/json", bytes.NewBuffer(openChargebackPayload))
	if err != nil {
		t.Fatalf("Failed to send chargeback webhook: %v", err)
	}
	openChargebackResp.Body.Close()

	if openChargebackResp.StatusCode != http.StatusOK && openChargebackResp.StatusCode != http.StatusCreated && openChargebackResp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 200, 201, or 202 for chargeback, got %d", openChargebackResp.StatusCode)
	}

	// Get the created dispute
	disputes := getAllDisputes(t, server)
	foundDispute := findDisputeByOrderID(disputes, orderID)
	if foundDispute == nil {
		t.Fatalf("Could not find dispute for order_id: %s", orderID)
	}

	disputeID := foundDispute["dispute_id"].(string)

	// Verify initial dispute status is "open"
	if foundDispute["status"] != "open" {
		t.Errorf("Expected initial dispute status to be 'open', got %v", foundDispute["status"])
	}

	// Add evidence to the dispute
	evidenceData := map[string]interface{}{
		"fields": map[string]string{
			"transaction_receipt":    "receipt_123",
			"customer_communication": "email_456",
			"shipping_tracking":      "track_789",
		},
		"files": []map[string]interface{}{
			{
				"file_id":      "file-1",
				"name":         "receipt.pdf",
				"content_type": "application/pdf",
				"size":         1024,
			},
			{
				"file_id":      "file-2",
				"name":         "communication.txt",
				"content_type": "text/plain",
				"size":         512,
			},
		},
	}

	evidencePayload, _ := json.Marshal(evidenceData)
	evidenceResp, err := http.Post(server.URL+"/disputes/"+disputeID+"/evidence", "application/json", bytes.NewBuffer(evidencePayload))
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}
	defer evidenceResp.Body.Close()

	if evidenceResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(evidenceResp.Body)
		t.Errorf("Expected status 200 for evidence addition, got %d. Response: %s", evidenceResp.StatusCode, string(body))
	}

	// Verify response contains evidence data
	var evidenceResult map[string]interface{}
	json.NewDecoder(evidenceResp.Body).Decode(&evidenceResult)

	if evidenceResult["dispute_id"] != disputeID {
		t.Errorf("Expected evidence dispute_id to be %s, got %v", disputeID, evidenceResult["dispute_id"])
	}

	// Verify evidence fields in response
	responseFields, ok := evidenceResult["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected fields to be a map, got %T", evidenceResult["fields"])
	}

	if responseFields["transaction_receipt"] != "receipt_123" {
		t.Errorf("Expected transaction_receipt to be 'receipt_123', got %v", responseFields["transaction_receipt"])
	}

	// Get updated disputes to verify status change
	updatedDisputes := getAllDisputes(t, server)
	updatedDispute := findDisputeByOrderID(updatedDisputes, orderID)
	if updatedDispute == nil {
		t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
	}

	// Verify dispute status changed to "under_review"
	if updatedDispute["status"] != "under_review" {
		t.Errorf("Expected dispute status to be 'under_review' after evidence addition, got %v", updatedDispute["status"])
	}

	// Database verification - get database connection from test setup
	cfg, err := config.New()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	pool, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(cfg.PgPoolMax))
	if err != nil {
		t.Fatalf("Failed to create postgres pool: %v", err)
	}
	defer pool.Close()

	// Verify evidence exists via API
	evidence := queryEvidenceFromAPI(t, server.URL, disputeID)
	if evidence == nil {
		t.Fatalf("Evidence not found for dispute_id: %s", disputeID)
	}

	// Verify evidence_added event was created
	events := queryDisputeEventsFromAPI(t, server.URL, disputeID)
	evidenceAddedEventFound := false
	for _, event := range events {
		if event["kind"] == "evidence_added" {
			evidenceAddedEventFound = true
			break
		}
	}

	if !evidenceAddedEventFound {
		t.Errorf("Expected evidence_added event to be created in dispute_events table")
	}
}

// Database query helper functions for evidence testing
func queryEvidenceFromDB(t *testing.T, pool *postgres.Postgres, disputeID string) map[string]interface{} {
	row := pool.Pool.QueryRow(context.Background(),
		"SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = $1",
		disputeID)

	var disputeIDResult, fieldsJSON, filesJSON string
	var updatedAt time.Time

	err := row.Scan(&disputeIDResult, &fieldsJSON, &filesJSON, &updatedAt)
	if err != nil {
		// Return nil if not found (expected for some test cases)
		return nil
	}

	var fields map[string]interface{}
	var files []interface{}

	json.Unmarshal([]byte(fieldsJSON), &fields)
	json.Unmarshal([]byte(filesJSON), &files)

	return map[string]interface{}{
		"dispute_id": disputeIDResult,
		"fields":     fields,
		"files":      files,
		"updated_at": updatedAt,
	}
}

func queryDisputeEventsFromAPI(t *testing.T, baseURL, disputeID string) []map[string]interface{} {
	url := fmt.Sprintf("%s/disputes/%s/events", baseURL, disputeID)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to query dispute events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var events []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("Failed to decode dispute events response: %v", err)
	}

	return events
}

func queryEvidenceFromAPI(t *testing.T, baseURL, disputeID string) map[string]interface{} {
	url := fmt.Sprintf("%s/disputes/%s/evidence", baseURL, disputeID)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to query evidence: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var evidence map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&evidence); err != nil {
		t.Fatalf("Failed to decode evidence response: %v", err)
	}

	return evidence
}
