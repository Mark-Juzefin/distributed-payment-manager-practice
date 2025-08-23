//go:build integration
// +build integration

package integration_test

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/app"
	apphttp "TestTaskJustPay/internal/controller/rest"
	"TestTaskJustPay/internal/controller/rest/handlers"
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/external/opensearch"
	"TestTaskJustPay/internal/external/silvergate"
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

var successfulSubmittingId = "sg-subm-1"

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
	//eventSink := eventsink.NewPgEventRepo(pool)
	opensearchEventSink, err := opensearch.NewOpenSearchEventSink(context.Background(), cfg.OpensearchUrls, cfg.OpensearchIndexDisputes, cfg.OpensearchIndexOrders)
	if err != nil {
		t.Fatalf("Failed to create opensearch event sink: %v", err)
	}

	orderService := order.NewOrderService(orderRepo)
	silvergateClient := silvergate.New(
		cfg.SilvergateBaseURL,
		cfg.SilvergateSubmitRepresentmentPath,
		&http.Client{Timeout: cfg.HTTPSilvergateClientTimeout},
	)
	disputeService := dispute.NewDisputeService(disputeRepo, silvergateClient, opensearchEventSink)

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

	createOrderPayload := map[string]interface{}{
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
	sendOrderWebhook(t, server, createOrderPayload)

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
	sendChargebacksWebhooks(t, server, openChargeback)

	// find one by order id
	foundDispute := findDisputeByOrderID(t, server.URL, orderID)
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

	sendChargebacksWebhooks(t, server, closeChargeback)

	// find updated dispute by order id
	updatedDispute := findDisputeByOrderID(t, server.URL, orderID)
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
		sendOrderWebhook(t, server, orderData)
	}

	// Get all orders to verify creation
	result := getOrders(t, server)

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

	sendOrderWebhook(t, server, initialOrder)

	// Verify order was created
	initialResult := getOrder(t, server, orderID)

	if initialResult["status"] != "created" {
		t.Errorf("Expected order status to be 'created', got %v", initialResult["status"])
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

	sendOrderWebhook(t, server, updatedOrder)

	// Verify order was updated
	updatedResult := getOrder(t, server, orderID)

	if updatedResult["status"] != "updated" {
		t.Errorf("Expected order status to be 'updated', got %v", updatedResult["status"])
	}
}

func TestEvidenceAdditionFlow(t *testing.T) {
	server := setupTestServer(t)
	defer server.Close()

	orderID := "order-evidence-test"

	disputeID := createOpenedDisputeForOrderId(t, server, orderID)

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

	evidenceResult := addEvidence(t, server, disputeID, evidenceData)

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
	updatedDispute := findDisputeByOrderID(t, server.URL, orderID)
	if updatedDispute == nil {
		t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
	}

	// Verify dispute status changed to "under_review"
	if updatedDispute["status"] != "under_review" {
		t.Errorf("Expected dispute status to be 'under_review' after evidence addition, got %v", updatedDispute["status"])
	}

	// Verify evidence exists via API
	evidence := getEvidence(t, server.URL, disputeID)
	if evidence == nil {
		t.Fatalf("Evidence not found for dispute_id: %s", disputeID)
	}

	// Verify evidence_added event was created
	events := getDisputeEvents(t, server.URL, disputeID)
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

func TestSubmitDisputeFlow(t *testing.T) {

	t.Run("Dispute with evidences is successfully submitted", func(t *testing.T) {
		server := setupTestServer(t)
		defer server.Close()

		orderID := "order-submit-test"

		// Create dispute
		disputeID := createOpenedDisputeForOrderId(t, server, orderID)

		// Verify initial dispute status is "open"
		initialDispute := findDisputeByOrderID(t, server.URL, orderID)
		if initialDispute == nil {
			t.Fatalf("Could not find dispute for order_id: %s", orderID)
		}
		if initialDispute["status"] != "open" {
			t.Errorf("Expected initial dispute status to be 'open', got %v", initialDispute["status"])
		}

		// Upload evidence
		evidenceData := map[string]interface{}{
			"fields": map[string]string{
				"transaction_receipt":    "receipt_submit_123",
				"customer_communication": "email_submit_456",
				"shipping_tracking":      "track_submit_789",
			},
			"files": []map[string]interface{}{
				{
					"file_id":      "submit-file-1",
					"name":         "submit_receipt.pdf",
					"content_type": "application/pdf",
					"size":         2048,
				},
			},
		}
		addEvidence(t, server, disputeID, evidenceData)

		// Verify status changed to "under_review" after evidence addition
		updatedDispute := findDisputeByOrderID(t, server.URL, orderID)
		if updatedDispute == nil {
			t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
		}
		if updatedDispute["status"] != "under_review" {
			t.Errorf("Expected dispute status to be 'under_review' after evidence addition, got %v", updatedDispute["status"])
		}

		// Submit dispute
		submitDispute(t, server, disputeID)

		// Verify final state after submission
		finalDispute := findDisputeByOrderID(t, server.URL, orderID)
		if finalDispute == nil {
			t.Fatalf("Could not find final dispute for order_id: %s", orderID)
		}

		// Verify status changed to "submitted"
		if finalDispute["status"] != "submitted" {
			t.Errorf("Expected dispute status to be 'submitted' after submission, got %v", finalDispute["status"])
		}

		// Verify submitted_at timestamp is set
		if finalDispute["submitted_at"] == nil {
			t.Errorf("Expected submitted_at to be set after submission")
		}

		// Verify submitting_id is set
		if finalDispute["submitting_id"] != successfulSubmittingId {
			t.Errorf("Expected submitting_id to be set after submission. Filan dispute %v", finalDispute)
		}

		// Verify evidence_submitted event was created
		events := getDisputeEvents(t, server.URL, disputeID)
		evidenceSubmittedEventFound := false
		for _, event := range events {
			if event["kind"] == "evidence_submitted" {
				evidenceSubmittedEventFound = true
				break
			}
		}

		if !evidenceSubmittedEventFound {
			t.Errorf("Expected evidence_submitted event to be created in dispute_events table")
		}
	})

}

func createOrderWithId(t *testing.T, server *httptest.Server, orderId string) {
	createOrderPayload := map[string]interface{}{
		"event_id":   "evt-order-evidence",
		"order_id":   orderId,
		"user_id":    "55555555-5555-5555-5555-555555555555",
		"status":     "created",
		"updated_at": time.Now().Format(time.RFC3339),
		"created_at": time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "250.75",
			"currency": "USD",
		},
	}

	sendOrderWebhook(t, server, createOrderPayload)
}

func sendOrderWebhook(t *testing.T, server *httptest.Server, payload map[string]interface{}) {
	orderPayload, _ := json.Marshal(payload)
	resp, err := http.Post(server.URL+"/webhooks/payments/orders", "application/json", bytes.NewBuffer(orderPayload))
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 200 or 201 for webhooks, got %d", resp.StatusCode)
	}
}

func sendChargebacksWebhooks(t *testing.T, server *httptest.Server, payload map[string]interface{}) {
	openChargebackPayload, _ := json.Marshal(payload)
	openChargebackResp, err := http.Post(server.URL+"/webhooks/payments/chargebacks", "application/json", bytes.NewBuffer(openChargebackPayload))
	if err != nil {
		t.Fatalf("Failed to send chargeback webhook: %v", err)
	}
	openChargebackResp.Body.Close()

	if openChargebackResp.StatusCode != http.StatusOK && openChargebackResp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 200, 201, or 202 for chargeback, got %d", openChargebackResp.StatusCode)
	}
}

func makeGetRequest(t *testing.T, url string, expectedStatus int) *http.Response {
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to make GET request to %s: %v", url, err)
	}

	if resp.StatusCode != expectedStatus {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Expected status %d, got %d. Response: %s", expectedStatus, resp.StatusCode, string(body))
	}

	return resp
}

func makeGetRequestWithResponse(t *testing.T, url string, expectedStatus int, target interface{}) {
	resp := makeGetRequest(t, url, expectedStatus)
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func makePostRequest(t *testing.T, url string, payload interface{}, expectedStatus int) *http.Response {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		t.Fatalf("Failed to make POST request to %s: %v", url, err)
	}

	if resp.StatusCode != expectedStatus {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Expected status %d, got %d. Response: %s", expectedStatus, resp.StatusCode, string(body))
	}

	return resp
}

func makePostRequestWithResponse(t *testing.T, url string, payload interface{}, expectedStatus int, target interface{}) {
	resp := makePostRequest(t, url, payload, expectedStatus)
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func getOrders(t *testing.T, server *httptest.Server) []map[string]interface{} {
	var orders []map[string]interface{}
	makeGetRequestWithResponse(t, server.URL+"/orders", http.StatusOK, &orders)
	return orders
}

func getOrder(t *testing.T, server *httptest.Server, orderID string) map[string]interface{} {
	var order map[string]interface{}
	makeGetRequestWithResponse(t, server.URL+"/orders/"+orderID, http.StatusOK, &order)
	return order
}

func getDisputes(t *testing.T, baseURL string) []map[string]interface{} {
	var disputes []map[string]interface{}
	makeGetRequestWithResponse(t, baseURL+"/disputes", http.StatusOK, &disputes)
	return disputes
}

func getDisputeEvents(t *testing.T, baseURL, disputeID string) []map[string]interface{} {
	var events []map[string]interface{}
	url := fmt.Sprintf("%s/disputes/%s/events", baseURL, disputeID)
	makeGetRequestWithResponse(t, url, http.StatusOK, &events)
	return events
}

func getEvidence(t *testing.T, baseURL, disputeID string) map[string]interface{} {
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

func addEvidence(t *testing.T, server *httptest.Server, disputeID string, evidenceData map[string]interface{}) map[string]interface{} {
	var evidenceResult map[string]interface{}
	url := server.URL + "/disputes/" + disputeID + "/evidence"
	makePostRequestWithResponse(t, url, evidenceData, http.StatusOK, &evidenceResult)
	return evidenceResult
}

func submitDispute(t *testing.T, server *httptest.Server, disputeID string) {
	url := server.URL + "/disputes/" + disputeID + "/submit"
	makePostRequest(t, url, nil, http.StatusAccepted)
}

func createOpenedDisputeForOrderId(t *testing.T, server *httptest.Server, orderId string) (disputeId string) {
	createOrderWithId(t, server, orderId)

	openChargeback := map[string]interface{}{
		"provider_event_id": "evt-evidence-1",
		"order_id":          orderId,
		"status":            "opened",
		"reason":            "unauthorized",
		"amount":            250.75,
		"currency":          "USD",
		"occurred_at":       time.Now().Format(time.RFC3339),
		"meta":              map[string]string{},
	}

	sendChargebacksWebhooks(t, server, openChargeback)

	foundDispute := findDisputeByOrderID(t, server.URL, orderId)
	if foundDispute == nil {
		t.Fatalf("Could not find dispute for order_id: %s", orderId)
	}

	disputeID := foundDispute["dispute_id"].(string)

	// Verify initial dispute status is "open"
	if foundDispute["status"] != "open" {
		t.Errorf("Expected initial dispute status to be 'open', got %v", foundDispute["status"])
	}
	return disputeID
}

func findDisputeByOrderID(t *testing.T, baseURL, orderID string) map[string]interface{} {
	disputes := getDisputes(t, baseURL)

	for _, disputeRecord := range disputes {
		if disputeRecord["order_id"] == orderID {
			return disputeRecord
		}
	}
	return nil
}
