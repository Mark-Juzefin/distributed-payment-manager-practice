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
	"TestTaskJustPay/internal/external/silvergate"
	dispute_repo "TestTaskJustPay/internal/repo/dispute"
	"TestTaskJustPay/internal/repo/eventsink"
	order_repo "TestTaskJustPay/internal/repo/order"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/google/go-querystring/query"
)

//go:embed testdata/orders-50_disputes-15_events-103.sql
var baseFixture string

func applyBaseFixture(t *testing.T, tx postgres.Executor) {
	t.Helper()
	_, err := tx.Exec(context.Background(), baseFixture)
	require.NoError(t, err)
}

var successfulSubmittingId = "sg-subm-1"

// todo: refactor
func setupTestServer(t *testing.T) (*httptest.Server, *postgres.Postgres) {
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
	_, err = pool.Pool.Exec(context.Background(), "TRUNCATE TABLE dispute_events, disputes, order_events, orders, evidence CASCADE")
	if err != nil {
		t.Fatalf("Failed to clean database: %v", err)
	}

	orderRepo := order_repo.NewPgOrderRepo(pool)
	disputeRepo := dispute_repo.NewPgDisputeRepo(pool)
	eventSink := eventsink.NewPgEventRepo(pool.Pool, pool.Builder)

	orderService := order.NewOrderService(orderRepo)
	silvergateClient := silvergate.New(
		cfg.SilvergateBaseURL,
		cfg.SilvergateSubmitRepresentmentPath,
		&http.Client{Timeout: cfg.HTTPSilvergateClientTimeout},
	)
	disputeService := dispute.NewDisputeService(disputeRepo, silvergateClient, eventSink)

	orderHandler := handlers.NewOrderHandler(orderService)
	chargebackHandler := handlers.NewChargebackHandler(disputeService)
	disputeHandler := handlers.NewDisputeHandler(disputeService)

	router := apphttp.NewRouter(orderHandler, chargebackHandler, disputeHandler)

	engine := app.NewGinEngine(l)
	router.SetUp(engine)

	return httptest.NewServer(engine), pool
}

func TestDisputePagination(t *testing.T) {
	server, pool := setupTestServer(t)
	defer server.Close()

	applyBaseFixture(t, pool.Pool)

	t.Run("Basic pagination flow", func(t *testing.T) {
		// dispute_001: 7, dispute_002: 7, dispute_003: 7, dispute_004: 6,
		// dispute_005: 8, dispute_006: 7, dispute_007: 8 = 50 total
		expectedTotal := 50
		targetDisputes := []string{"dispute_001", "dispute_002", "dispute_003", "dispute_004", "dispute_005", "dispute_006", "dispute_007"}

		filter := dispute.DisputeEventQuery{
			DisputeIDs: targetDisputes,
			Limit:      20,
			Cursor:     "",
			SortAsc:    true,
		}

		// First page: 20 items
		page1 := getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page1.Items, 20)
		assert.True(t, page1.HasMore)
		assert.NotEmpty(t, page1.NextCursor)

		// Verify events are ordered by creation time
		for i := 1; i < len(page1.Items); i++ {
			assert.True(t, page1.Items[i-1].CreatedAt.Before(page1.Items[i].CreatedAt) ||
				page1.Items[i-1].CreatedAt.Equal(page1.Items[i].CreatedAt),
				"Events should be ordered by creation time")
		}

		// Second page: 20 items
		filter.Cursor = page1.NextCursor
		page2 := getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page2.Items, 20)
		assert.True(t, page2.HasMore)
		assert.NotEmpty(t, page2.NextCursor)
		assert.NotEqual(t, page1.NextCursor, page2.NextCursor, "Cursors should be different")

		// Third page: 10 items
		filter.Cursor = page2.NextCursor
		page3 := getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page3.Items, 10)
		assert.False(t, page3.HasMore)
		assert.Empty(t, page3.NextCursor)

		// Verify no duplicate events across pages
		allEventIDs := make(map[string]bool)
		for _, event := range append(append(page1.Items, page2.Items...), page3.Items...) {
			assert.False(t, allEventIDs[event.EventID], "Event ID should be unique: %s", event.EventID)
			allEventIDs[event.EventID] = true
		}
		assert.Len(t, allEventIDs, expectedTotal, "Total unique events should match expected")
	})

	t.Run("Should sort ASC/DESC", func(t *testing.T) {
		// dispute_001: 7
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_001"},
			Limit:      10, // default
			Cursor:     "",
			SortAsc:    true,
		}

		page := getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page.Items, 7)

		// Verify events are ordered by creation time ASC
		for i := 1; i < len(page.Items); i++ {
			assert.True(t, page.Items[i-1].CreatedAt.Before(page.Items[i].CreatedAt) ||
				page.Items[i-1].CreatedAt.Equal(page.Items[i].CreatedAt),
				"Events should be ordered by creation time")
		}

		filter.SortAsc = false

		page = getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page.Items, 7)

		// Verify events are ordered by creation time DESC
		for i := 1; i < len(page.Items); i++ {
			assert.True(t, page.Items[i-1].CreatedAt.After(page.Items[i].CreatedAt) ||
				page.Items[i-1].CreatedAt.Equal(page.Items[i].CreatedAt),
				"Events should be ordered by creation time")
		}
	})

	t.Run("Single page result", func(t *testing.T) {
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_001"}, // Only 7 events
			Limit:      20,
			Cursor:     "",
		}

		page := getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page.Items, 7) // dispute_001 has exactly 7 events
		assert.False(t, page.HasMore)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("Small limit pagination", func(t *testing.T) {
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_001", "dispute_002"}, // 14 total events
			Limit:      5,
			Cursor:     "",
		}

		var allEvents []dispute.DisputeEvent
		pageCount := 0

		for {
			page := getDisputeEvents(t, server.URL, filter)
			allEvents = append(allEvents, page.Items...)
			pageCount++

			if !page.HasMore {
				assert.Empty(t, page.NextCursor)
				break
			}

			assert.NotEmpty(t, page.NextCursor)
			filter.Cursor = page.NextCursor

			if pageCount > 3 {
				t.Fatal("Too many pages")
			}
		}

		assert.Len(t, allEvents, 14)  // dispute_001 (7) + dispute_002 (7)
		assert.Equal(t, pageCount, 3) // ceil(14/5) = 3 pages
	})

	t.Run("Empty result pagination", func(t *testing.T) {
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_999"}, // Non-existent dispute
			Limit:      10,
		}

		page := getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page.Items, 0)
		assert.False(t, page.HasMore)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("Event kind filtering with pagination", func(t *testing.T) {
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_011", "dispute_012", "dispute_013", "dispute_014", "dispute_015"},
			Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventEvidenceAdded},
			Limit:      20,
		}

		page := getDisputeEvents(t, server.URL, filter)
		assert.Len(t, page.Items, 14)
	})
}

func TestChargebackFlow(t *testing.T) {
	server, _ := setupTestServer(t)
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

	if foundDispute.Status != "open" {
		t.Errorf("Expected dispute status to be 'open', got %v", foundDispute.Status)
	}
	if foundDispute.Reason != "fraud" {
		t.Errorf("Expected dispute reason to be 'fraud', got %v", foundDispute.Reason)
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

	if updatedDispute.Status != "won" {
		t.Errorf("Expected updated dispute status to be 'won', got %v", updatedDispute.Status)
	}

	// verify closed_at timestamp is set
	if updatedDispute.ClosedAt == nil {
		t.Errorf("Expected closed_at to be set for closed dispute")
	}
}

func TestCreateOrdersFlow(t *testing.T) {
	server, _ := setupTestServer(t)
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
	server, _ := setupTestServer(t)
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

	if initialResult.Status != "created" {
		t.Errorf("Expected order status to be 'created', got %v", initialResult.Status)
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

	if updatedResult.Status != "updated" {
		t.Errorf("Expected order status to be 'updated', got %v", updatedResult.Status)
	}
}

func TestEvidenceAdditionFlow(t *testing.T) {
	server, _ := setupTestServer(t)
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

	if evidenceResult.DisputeID != disputeID {
		t.Errorf("Expected evidence dispute_id to be %s, got %v", disputeID, evidenceResult.DisputeID)
	}

	// Verify evidence fields in response
	if evidenceResult.Fields["transaction_receipt"] != "receipt_123" {
		t.Errorf("Expected transaction_receipt to be 'receipt_123', got %v", evidenceResult.Fields["transaction_receipt"])
	}

	// Get updated disputes to verify status change
	updatedDispute := findDisputeByOrderID(t, server.URL, orderID)
	if updatedDispute == nil {
		t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
	}

	// Verify dispute status changed to "under_review"
	if updatedDispute.Status != "under_review" {
		t.Errorf("Expected dispute status to be 'under_review' after evidence addition, got %v", updatedDispute.Status)
	}

	// Verify evidence exists via API
	evidence := getEvidence(t, server.URL, disputeID)
	if evidence == nil {
		t.Fatalf("Evidence not found for dispute_id: %s", disputeID)
	}

	// Verify evidence_added event was created
	page := getDisputeEvents(t, server.URL, dispute.DisputeEventQuery{
		DisputeIDs: []string{disputeID},
		Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventEvidenceAdded},
	})

	if len(page.Items) == 0 {
		t.Errorf("Expected evidence_submitted event to be created in dispute_events table")
	}
}

func TestSubmitDisputeFlow(t *testing.T) {

	t.Run("Dispute with evidences is successfully submitted", func(t *testing.T) {
		server, _ := setupTestServer(t)
		defer server.Close()

		orderID := "order-submit-test"

		// Create dispute
		disputeID := createOpenedDisputeForOrderId(t, server, orderID)

		// Verify initial dispute status is "open"
		initialDispute := findDisputeByOrderID(t, server.URL, orderID)
		if initialDispute == nil {
			t.Fatalf("Could not find dispute for order_id: %s", orderID)
		}
		if initialDispute.Status != "open" {
			t.Errorf("Expected initial dispute status to be 'open', got %v", initialDispute.Status)
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
		if updatedDispute.Status != "under_review" {
			t.Errorf("Expected dispute status to be 'under_review' after evidence addition, got %v", updatedDispute.Status)
		}

		// Submit dispute
		submitDispute(t, server, disputeID)

		// Verify final state after submission
		finalDispute := findDisputeByOrderID(t, server.URL, orderID)
		if finalDispute == nil {
			t.Fatalf("Could not find final dispute for order_id: %s", orderID)
		}

		// Verify status changed to "submitted"
		if finalDispute.Status != "submitted" {
			t.Errorf("Expected dispute status to be 'submitted' after submission, got %v", finalDispute.Status)
		}

		// Verify submitted_at timestamp is set
		if finalDispute.SubmittedAt == nil {
			t.Errorf("Expected submitted_at to be set after submission")
		}

		// Verify submitting_id is set
		if finalDispute.SubmittingId == nil || *finalDispute.SubmittingId != successfulSubmittingId {
			t.Errorf("Expected submitting_id to be set after submission. Filan dispute %v", finalDispute)
		}

		// Verify evidence_submitted event was created
		page := getDisputeEvents(t, server.URL, dispute.DisputeEventQuery{
			DisputeIDs: []string{disputeID},
			Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventEvidenceSubmitted},
		})

		if len(page.Items) == 0 {
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

func GET[T any](t *testing.T, baseUrl, path string, queryPayload any, expectedStatus int) T {
	t.Helper()

	var res T

	u, _ := url.Parse(baseUrl)
	u.Path = path
	if queryPayload != nil {
		v, _ := query.Values(queryPayload)
		u.RawQuery = v.Encode()
	}

	resp, err := http.Get(u.String())
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, expectedStatus, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&res)
	require.NoError(t, err)
	return res
}

func POST[T any](t *testing.T, baseUrl, path string, payload any, expectedStatus int) T {
	t.Helper()

	var res T

	u, _ := url.Parse(baseUrl)
	u.Path = path

	var reqBody *bytes.Buffer
	if payload != nil {
		jsonPayload, err := json.Marshal(payload)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(jsonPayload)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	resp, err := http.Post(u.String(), "application/json", reqBody)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, expectedStatus, resp.StatusCode)

	if resp.ContentLength > 0 {
		err = json.NewDecoder(resp.Body).Decode(&res)
		require.NoError(t, err)
	}

	return res
}

func getOrders(t *testing.T, server *httptest.Server) []order.Order {
	orders := GET[[]order.Order](t, server.URL, "/orders", nil, http.StatusOK)
	return orders
}

func getOrder(t *testing.T, server *httptest.Server, orderID string) order.Order {
	order := GET[order.Order](t, server.URL, "/orders/"+orderID, nil, http.StatusOK)
	return order
}

func getDisputes(t *testing.T, baseURL string) []dispute.Dispute {
	disputes := GET[[]dispute.Dispute](t, baseURL, "/disputes", nil, http.StatusOK)
	return disputes
}

func getDisputeEvents(t *testing.T, baseURL string, query dispute.DisputeEventQuery) dispute.DisputeEventPage {
	page := GET[dispute.DisputeEventPage](t, baseURL, "/disputes/events", query, http.StatusOK)
	return page
}

func getEvidence(t *testing.T, baseURL, disputeID string) *dispute.Evidence {
	evidence := GET[dispute.Evidence](t, baseURL, "/disputes/"+disputeID+"/evidence", nil, http.StatusOK)
	return &evidence
}

func addEvidence(t *testing.T, server *httptest.Server, disputeID string, evidenceData map[string]interface{}) dispute.Evidence {
	evidenceResult := POST[dispute.Evidence](t, server.URL, "/disputes/"+disputeID+"/evidence", evidenceData, http.StatusOK)
	return evidenceResult
}

func submitDispute(t *testing.T, server *httptest.Server, disputeID string) {
	POST[any](t, server.URL, "/disputes/"+disputeID+"/submit", nil, http.StatusAccepted)
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

	disputeID := foundDispute.ID

	// Verify initial dispute status is "open"
	if foundDispute.Status != "open" {
		t.Errorf("Expected initial dispute status to be 'open', got %v", foundDispute.Status)
	}
	return disputeID
}

func findDisputeByOrderID(t *testing.T, baseURL, orderID string) *dispute.Dispute {
	disputes := getDisputes(t, baseURL)

	for _, disputeRecord := range disputes {
		if disputeRecord.OrderID == orderID {
			return &disputeRecord
		}
	}
	return nil
}
