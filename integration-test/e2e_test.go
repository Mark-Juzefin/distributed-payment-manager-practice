//go:build integration

package integration_test

import (
	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_DisputePagination(t *testing.T) {
	truncateDB(t)
	applyBaseFixture(t)

	t.Run("Basic pagination flow", func(t *testing.T) {
		expectedTotal := 50
		targetDisputes := []string{"dispute_001", "dispute_002", "dispute_003", "dispute_004", "dispute_005", "dispute_006", "dispute_007"}

		filter := dispute.DisputeEventQuery{
			DisputeIDs: targetDisputes,
			Limit:      20,
			Cursor:     "",
			SortAsc:    true,
		}

		// First page: 20 items
		page1 := getDisputeEvents(t, filter)
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
		page2 := getDisputeEvents(t, filter)
		assert.Len(t, page2.Items, 20)
		assert.True(t, page2.HasMore)
		assert.NotEmpty(t, page2.NextCursor)
		assert.NotEqual(t, page1.NextCursor, page2.NextCursor, "Cursors should be different")

		// Third page: 10 items
		filter.Cursor = page2.NextCursor
		page3 := getDisputeEvents(t, filter)
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
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_001"},
			Limit:      10,
			Cursor:     "",
			SortAsc:    true,
		}

		page := getDisputeEvents(t, filter)
		assert.Len(t, page.Items, 7)

		for i := 1; i < len(page.Items); i++ {
			assert.True(t, page.Items[i-1].CreatedAt.Before(page.Items[i].CreatedAt) ||
				page.Items[i-1].CreatedAt.Equal(page.Items[i].CreatedAt),
				"Events should be ordered by creation time")
		}

		filter.SortAsc = false

		page = getDisputeEvents(t, filter)
		assert.Len(t, page.Items, 7)

		for i := 1; i < len(page.Items); i++ {
			assert.True(t, page.Items[i-1].CreatedAt.After(page.Items[i].CreatedAt) ||
				page.Items[i-1].CreatedAt.Equal(page.Items[i].CreatedAt),
				"Events should be ordered by creation time")
		}
	})

	t.Run("Single page result", func(t *testing.T) {
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_001"},
			Limit:      20,
			Cursor:     "",
		}

		page := getDisputeEvents(t, filter)
		assert.Len(t, page.Items, 7)
		assert.False(t, page.HasMore)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("Small limit pagination", func(t *testing.T) {
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_001", "dispute_002"},
			Limit:      5,
			Cursor:     "",
		}

		var allEvents []dispute.DisputeEvent
		pageCount := 0

		for {
			page := getDisputeEvents(t, filter)
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

		assert.Len(t, allEvents, 14)
		assert.Equal(t, pageCount, 3)
	})

	t.Run("Empty result pagination", func(t *testing.T) {
		filter := dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute_999"},
			Limit:      10,
		}

		page := getDisputeEvents(t, filter)
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

		page := getDisputeEvents(t, filter)
		assert.Len(t, page.Items, 14)
	})
}

func TestE2E_ChargebackFlow(t *testing.T) {
	truncateDB(t)

	orderID := "order-chargeback-1"

	createOrderPayload := map[string]interface{}{
		"provider_event_id": "evt-order-1",
		"order_id":          orderID,
		"user_id":           "44444444-4444-4444-4444-444444444444",
		"status":            "created",
		"updated_at":        time.Now().Format(time.RFC3339),
		"created_at":        time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "100.50",
			"currency": "USD",
		},
	}
	sendOrderWebhook(t, createOrderPayload)

	if waitForOrder(t, orderID, 40) == nil {
		t.Fatalf("Order was not created: %s", orderID)
	}

	// Create chargeback to trigger dispute creation
	openChargeback := map[string]interface{}{
		"provider_event_id": "evt-1",
		"order_id":          orderID,
		"user_id":           "44444444-4444-4444-4444-444444444444",
		"status":            "opened",
		"reason":            "fraud",
		"amount":            100.50,
		"currency":          "USD",
		"occurred_at":       time.Now().Format(time.RFC3339),
		"meta":              map[string]string{},
	}

	sendChargebackWebhook(t, openChargeback)

	foundDispute := waitForDisputeByOrderID(t, orderID, 15)
	if foundDispute == nil {
		t.Fatalf("Could not find dispute for order_id: %s", orderID)
	}

	if foundDispute.Status != "open" {
		t.Errorf("Expected dispute status to be 'open', got %v", foundDispute.Status)
	}
	if foundDispute.Reason != "fraud" {
		t.Errorf("Expected dispute reason to be 'fraud', got %v", foundDispute.Reason)
	}

	// Close dispute as won
	closeChargeback := map[string]interface{}{
		"provider_event_id": "evt-2",
		"order_id":          orderID,
		"user_id":           "44444444-4444-4444-4444-444444444444",
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

	sendChargebackWebhook(t, closeChargeback)

	updatedDispute := waitForDisputeStatus(t, orderID, "won", 15)
	if updatedDispute == nil {
		t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
	}

	if updatedDispute.Status != "won" {
		t.Errorf("Expected updated dispute status to be 'won', got %v", updatedDispute.Status)
	}

	if updatedDispute.ClosedAt == nil {
		t.Errorf("Expected closed_at to be set for closed dispute")
	}
}

func TestE2E_CreateOrdersFlow(t *testing.T) {
	truncateDB(t)

	orderId1 := "order-1"
	orderId2 := "order-2"

	orders := []map[string]interface{}{
		{
			"provider_event_id": "evt-1",
			"order_id":          orderId1,
			"user_id":           "11111111-1111-1111-1111-111111111111",
			"status":            "created",
			"updated_at":        time.Now().Format(time.RFC3339),
			"created_at":        time.Now().Format(time.RFC3339),
			"meta": map[string]string{
				"amount":   "100.50",
				"currency": "USD",
			},
		},
		{
			"provider_event_id": "evt-2",
			"order_id":          orderId2,
			"user_id":           "22222222-2222-2222-2222-222222222222",
			"status":            "created",
			"updated_at":        time.Now().Format(time.RFC3339),
			"created_at":        time.Now().Format(time.RFC3339),
			"meta": map[string]string{
				"amount":   "200.75",
				"currency": "EUR",
			},
		},
	}

	for _, orderData := range orders {
		sendOrderWebhook(t, orderData)
	}

	if waitForOrder(t, orderId1, 40) == nil {
		t.Fatalf("Order %s was not created in time", orderId1)
	}

	if waitForOrder(t, orderId2, 40) == nil {
		t.Fatalf("Order %s was not created in time", orderId2)
	}

	result := getOrders(t)

	if len(result) < 2 {
		t.Errorf("Expected at least 2 orders, got %d", len(result))
	}

	eventsPage := getOrderEvents(t, order.OrderEventQuery{
		OrderIDs: []string{orderId1, orderId2},
		Kinds:    []order.OrderEventKind{order.OrderEventWebhookReceived},
		Limit:    10,
	})
	assert.Len(t, eventsPage.Items, 2, "Should have webhook_received events for both orders")
}

func TestE2E_UpdateOrderFlow(t *testing.T) {
	truncateDB(t)

	orderID := "order-update-test"

	initialOrder := map[string]interface{}{
		"provider_event_id": "evt-create",
		"order_id":          orderID,
		"user_id":           "33333333-3333-3333-3333-333333333333",
		"status":            "created",
		"updated_at":        time.Now().Format(time.RFC3339),
		"created_at":        time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "150.25",
			"currency": "USD",
		},
	}

	sendOrderWebhook(t, initialOrder)

	initialResult := waitForOrderStatus(t, orderID, "created", 20)
	if initialResult == nil || initialResult.Status != "created" {
		t.Fatalf("Expected order status to be 'created', got %v", initialResult)
	}

	updatedOrder := map[string]interface{}{
		"provider_event_id": "evt-update",
		"order_id":          orderID,
		"user_id":           "33333333-3333-3333-3333-333333333333",
		"status":            "updated",
		"updated_at":        time.Now().Format(time.RFC3339),
		"created_at":        time.Now().Add(-time.Hour).Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "150.25",
			"currency": "USD",
		},
	}

	sendOrderWebhook(t, updatedOrder)

	updatedResult := waitForOrderStatus(t, orderID, "updated", 20)
	if updatedResult == nil || updatedResult.Status != "updated" {
		t.Errorf("Expected order status to be 'updated', got %v", updatedResult)
	}
}

func TestE2E_EvidenceAdditionFlow(t *testing.T) {
	truncateDB(t)

	orderID := "order-evidence-test"

	disputeID := createOpenedDisputeForOrderId(t, orderID)

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

	evidenceResult := addEvidence(t, disputeID, evidenceData)

	if evidenceResult.DisputeID != disputeID {
		t.Errorf("Expected evidence dispute_id to be %s, got %v", disputeID, evidenceResult.DisputeID)
	}

	if evidenceResult.Fields["transaction_receipt"] != "receipt_123" {
		t.Errorf("Expected transaction_receipt to be 'receipt_123', got %v", evidenceResult.Fields["transaction_receipt"])
	}

	updatedDispute := waitForDisputeStatus(t, orderID, "under_review", 15)
	if updatedDispute == nil {
		t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
	}

	if updatedDispute.Status != "under_review" {
		t.Errorf("Expected dispute status to be 'under_review' after evidence addition, got %v", updatedDispute.Status)
	}

	evidence := getEvidence(t, disputeID)
	if evidence == nil {
		t.Fatalf("Evidence not found for dispute_id: %s", disputeID)
	}

	page := getDisputeEvents(t, dispute.DisputeEventQuery{
		DisputeIDs: []string{disputeID},
		Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventEvidenceAdded},
	})

	if len(page.Items) == 0 {
		t.Errorf("Expected evidence_submitted event to be created in dispute_events table")
	}
}

func TestE2E_SubmitDisputeFlow(t *testing.T) {
	t.Run("Dispute with evidences is successfully submitted", func(t *testing.T) {
		truncateDB(t)

		orderID := "order-submit-test"

		disputeID := createOpenedDisputeForOrderId(t, orderID)

		initialDispute := waitForDisputeByOrderID(t, orderID, 15)
		if initialDispute == nil {
			t.Fatalf("Could not find dispute for order_id: %s", orderID)
		}
		if initialDispute.Status != "open" {
			t.Errorf("Expected initial dispute status to be 'open', got %v", initialDispute.Status)
		}

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
		addEvidence(t, disputeID, evidenceData)

		updatedDispute := waitForDisputeStatus(t, orderID, "under_review", 15)
		if updatedDispute == nil {
			t.Fatalf("Could not find updated dispute for order_id: %s", orderID)
		}
		if updatedDispute.Status != "under_review" {
			t.Errorf("Expected dispute status to be 'under_review' after evidence addition, got %v", updatedDispute.Status)
		}

		submitDispute(t, disputeID)

		finalDispute := waitForDisputeStatus(t, orderID, "submitted", 15)
		if finalDispute == nil {
			t.Fatalf("Could not find final dispute for order_id: %s", orderID)
		}

		if finalDispute.Status != "submitted" {
			t.Errorf("Expected dispute status to be 'submitted' after submission, got %v", finalDispute.Status)
		}

		if finalDispute.SubmittedAt == nil {
			t.Errorf("Expected submitted_at to be set after submission")
		}

		if finalDispute.SubmittingId == nil || *finalDispute.SubmittingId != successfulSubmittingId {
			t.Errorf("Expected submitting_id to be set after submission. Final dispute %v", finalDispute)
		}

		page := getDisputeEvents(t, dispute.DisputeEventQuery{
			DisputeIDs: []string{disputeID},
			Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventEvidenceSubmitted},
		})

		if len(page.Items) == 0 {
			t.Errorf("Expected evidence_submitted event to be created in dispute_events table")
		}
	})
}

func TestE2E_OrderHoldFlow(t *testing.T) {
	truncateDB(t)

	orderID := "order-hold-test"

	createOrderWithId(t, orderID)

	initialOrder := getOrder(t, orderID)
	assert.Equal(t, "created", string(initialOrder.Status))
	require.False(t, initialOrder.OnHold)
	require.Nil(t, initialOrder.HoldReason)

	// Set order on hold with manual_review reason
	holdRequest := map[string]interface{}{
		"action": "set",
		"reason": "manual_review",
	}
	holdResp := POST[order.HoldResponse](t, apiURL(), "/orders/"+orderID+"/hold", holdRequest, http.StatusOK)
	require.Equal(t, orderID, holdResp.OrderID)
	require.True(t, holdResp.OnHold)
	require.NotNil(t, holdResp.Reason)
	require.Equal(t, "manual_review", *holdResp.Reason)

	// Verify order is on hold
	heldOrder := getOrder(t, orderID)
	require.True(t, heldOrder.OnHold)
	require.NotNil(t, heldOrder.HoldReason)
	require.Equal(t, "manual_review", *heldOrder.HoldReason)

	// Clear order hold
	clearRequest := map[string]interface{}{
		"action": "clear",
	}
	clearResp := POST[order.HoldResponse](t, apiURL(), "/orders/"+orderID+"/hold", clearRequest, http.StatusOK)
	require.Equal(t, orderID, clearResp.OrderID)
	require.False(t, clearResp.OnHold)
	require.Nil(t, clearResp.Reason)

	// Verify order hold is cleared
	clearedOrder := getOrder(t, orderID)
	require.False(t, clearedOrder.OnHold)
	require.Nil(t, clearedOrder.HoldReason)

	// Test setting hold with risk reason
	riskRequest := map[string]interface{}{
		"action": "set",
		"reason": "risk",
	}
	riskResp := POST[order.HoldResponse](t, apiURL(), "/orders/"+orderID+"/hold", riskRequest, http.StatusOK)
	require.Equal(t, orderID, riskResp.OrderID)
	require.True(t, riskResp.OnHold)
	require.NotNil(t, riskResp.Reason)
	require.Equal(t, "risk", *riskResp.Reason)

	// Verify hold-related events
	eventsPage := getOrderEvents(t, order.OrderEventQuery{
		OrderIDs: []string{orderID},
		Limit:    10,
	})
	require.Len(t, eventsPage.Items, 4, "Should have 4 events: webhook_received, hold_set, hold_cleared, hold_set")

	eventKinds := make([]string, len(eventsPage.Items))
	for i, event := range eventsPage.Items {
		eventKinds[i] = string(event.Kind)
	}
	expectedKinds := []string{"webhook_received", "hold_set", "hold_cleared", "hold_set"}
	assert.ElementsMatch(t, expectedKinds, eventKinds)
}

func TestE2E_OrderCaptureFlow(t *testing.T) {
	truncateDB(t)

	orderID := "order-capture-test"

	createOrderWithId(t, orderID)

	initialOrder := getOrder(t, orderID)
	require.Equal(t, "created", string(initialOrder.Status))
	require.False(t, initialOrder.OnHold)

	// Test successful capture
	captureRequest := map[string]interface{}{
		"amount":          100.50,
		"currency":        "USD",
		"idempotency_key": "capture-test-key-1",
	}

	captureResp := POST[order.CaptureResponse](t, apiURL(), "/orders/"+orderID+"/capture", captureRequest, http.StatusOK)
	require.Equal(t, orderID, captureResp.OrderID)
	require.Equal(t, 100.50, captureResp.Amount)
	require.Equal(t, "USD", captureResp.Currency)
	require.Equal(t, "success", captureResp.Status)
	require.Equal(t, "txn-capture-123", captureResp.ProviderTxID)
	require.NotZero(t, captureResp.CapturedAt)

	// Verify capture events
	eventsPage := getOrderEvents(t, order.OrderEventQuery{
		OrderIDs: []string{orderID},
		Limit:    10,
	})
	require.Len(t, eventsPage.Items, 3, "Should have 3 events: webhook_received, capture_requested, capture_completed")

	eventKinds := make([]string, len(eventsPage.Items))
	for i, event := range eventsPage.Items {
		eventKinds[i] = string(event.Kind)
	}
	expectedKinds := []string{"webhook_received", "capture_requested", "capture_completed"}
	assert.ElementsMatch(t, expectedKinds, eventKinds)

	// Test capture on held order should fail
	heldOrderID := "order-held-capture-test"
	createOrderWithId(t, heldOrderID)

	holdRequest := map[string]interface{}{
		"action": "set",
		"reason": "manual_review",
	}
	POST[order.HoldResponse](t, apiURL(), "/orders/"+heldOrderID+"/hold", holdRequest, http.StatusOK)

	// Try to capture held order — should fail with 409
	heldCaptureRequest := map[string]interface{}{
		"amount":          50.25,
		"currency":        "USD",
		"idempotency_key": "capture-held-key-1",
	}

	resp, err := http.Post(apiURL()+"/orders/"+heldOrderID+"/capture", "application/json",
		bytes.NewBuffer(func() []byte {
			b, _ := json.Marshal(heldCaptureRequest)
			return b
		}()))
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestE2E_OrderEventsFlow(t *testing.T) {
	truncateDB(t)

	t.Run("Order events pagination and filtering", func(t *testing.T) {
		orderIDs := []string{"order-events-1", "order-events-2", "order-events-3"}

		for _, oid := range orderIDs {
			createOrderWithId(t, oid)

			holdRequest := map[string]interface{}{
				"action": "set",
				"reason": "risk",
			}
			POST[order.HoldResponse](t, apiURL(), "/orders/"+oid+"/hold", holdRequest, http.StatusOK)

			clearRequest := map[string]interface{}{
				"action": "clear",
			}
			POST[order.HoldResponse](t, apiURL(), "/orders/"+oid+"/hold", clearRequest, http.StatusOK)
		}

		// Test filtering by event kind
		holdEventsPage := getOrderEvents(t, order.OrderEventQuery{
			OrderIDs: orderIDs,
			Kinds:    []order.OrderEventKind{order.OrderEventHoldSet},
			Limit:    10,
		})

		assert.Len(t, holdEventsPage.Items, 3)
		for _, event := range holdEventsPage.Items {
			assert.Equal(t, order.OrderEventHoldSet, event.Kind)
		}

		// Test pagination with small limit
		allEventsPage := getOrderEvents(t, order.OrderEventQuery{
			OrderIDs: orderIDs,
			Limit:    5,
		})

		assert.Len(t, allEventsPage.Items, 5)
		assert.True(t, allEventsPage.HasMore)
		assert.NotEmpty(t, allEventsPage.NextCursor)

		// Get second page
		nextPage := getOrderEvents(t, order.OrderEventQuery{
			OrderIDs: orderIDs,
			Limit:    5,
			Cursor:   allEventsPage.NextCursor,
		})

		assert.Len(t, nextPage.Items, 4)
		assert.False(t, nextPage.HasMore)
		assert.Empty(t, nextPage.NextCursor)
	})

	t.Run("Order events sorting", func(t *testing.T) {
		orderID := "order-events-sort-test"
		createOrderWithId(t, orderID)

		time.Sleep(10 * time.Millisecond)
		holdRequest := map[string]interface{}{
			"action": "set",
			"reason": "manual_review",
		}
		POST[order.HoldResponse](t, apiURL(), "/orders/"+orderID+"/hold", holdRequest, http.StatusOK)

		time.Sleep(10 * time.Millisecond)
		clearRequest := map[string]interface{}{
			"action": "clear",
		}
		POST[order.HoldResponse](t, apiURL(), "/orders/"+orderID+"/hold", clearRequest, http.StatusOK)

		// Test ascending sort
		ascPage := getOrderEvents(t, order.OrderEventQuery{
			OrderIDs: []string{orderID},
			SortAsc:  true,
			Limit:    10,
		})

		for i := 1; i < len(ascPage.Items); i++ {
			assert.True(t,
				ascPage.Items[i-1].CreatedAt.Before(ascPage.Items[i].CreatedAt) ||
					ascPage.Items[i-1].CreatedAt.Equal(ascPage.Items[i].CreatedAt),
				"Events should be ordered by creation time ASC")
		}

		// Test descending sort
		descPage := getOrderEvents(t, order.OrderEventQuery{
			OrderIDs: []string{orderID},
			SortAsc:  false,
			Limit:    10,
		})

		for i := 1; i < len(descPage.Items); i++ {
			assert.True(t,
				descPage.Items[i-1].CreatedAt.After(descPage.Items[i].CreatedAt) ||
					descPage.Items[i-1].CreatedAt.Equal(descPage.Items[i].CreatedAt),
				"Events should be ordered by creation time DESC")
		}
	})
}
