//go:build integration

package integration_test

import (
	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/orders-50_disputes-15_events-103.sql
var baseFixture string

var successfulSubmittingId = "sg-subm-1"

// truncateDB clears all tables for test isolation.
func truncateDB(t *testing.T) {
	t.Helper()
	err := suite.Postgres.Truncate(context.Background())
	require.NoError(t, err, "failed to truncate database")
}

// applyBaseFixture loads the standard test fixture into the database.
func applyBaseFixture(t *testing.T) {
	t.Helper()
	_, err := suite.Postgres.Pool.Pool.Exec(context.Background(), baseFixture)
	require.NoError(t, err, "failed to apply base fixture")
}

// E2E URL helpers — webhooks go to Ingest, queries go to API
func ingestURL() string { return suite.Ingest.BaseURL }
func apiURL() string    { return suite.API.BaseURL }

// retryGet retries GET request on 404 for eventual consistency (async processing).
func retryGet(t *testing.T, doRequest func() (*http.Response, error), maxRetries int) *http.Response {
	t.Helper()
	var resp *http.Response
	var err error

	for i := 0; i < maxRetries; i++ {
		resp, err = doRequest()
		require.NoError(t, err)

		if resp.StatusCode != http.StatusNotFound {
			return resp
		}

		resp.Body.Close()
		time.Sleep(200 * time.Millisecond)
	}

	return resp
}

// --- HTTP helpers ---

func GET[T any](t *testing.T, baseUrl, path string, queryPayload any, expectedStatus int) T {
	t.Helper()

	var res T

	u, _ := url.Parse(baseUrl)
	u.Path = path
	if queryPayload != nil {
		v, _ := query.Values(queryPayload)
		u.RawQuery = v.Encode()
	}

	// Use retry logic for eventual consistency (async processing)
	resp := retryGet(t, func() (*http.Response, error) {
		return http.Get(u.String())
	}, 10)
	defer resp.Body.Close()

	require.Equal(t, expectedStatus, resp.StatusCode)

	err := json.NewDecoder(resp.Body).Decode(&res)
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

// --- Domain-specific helpers ---

func sendOrderWebhook(t *testing.T, payload map[string]interface{}) {
	t.Helper()
	orderPayload, _ := json.Marshal(payload)
	resp, err := http.Post(ingestURL()+"/webhooks/payments/orders", "application/json", bytes.NewBuffer(orderPayload))
	require.NoError(t, err)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 200, 201, or 202 for webhooks, got %d", resp.StatusCode)
	}
}

func sendChargebackWebhook(t *testing.T, payload map[string]interface{}) {
	t.Helper()
	chargebackPayload, _ := json.Marshal(payload)
	resp, err := http.Post(ingestURL()+"/webhooks/payments/chargebacks", "application/json", bytes.NewBuffer(chargebackPayload))
	require.NoError(t, err)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status 200 or 202 for chargeback, got %d", resp.StatusCode)
	}
}

func getOrders(t *testing.T) []order.Order {
	return GET[[]order.Order](t, apiURL(), "/orders", nil, http.StatusOK)
}

func getOrder(t *testing.T, orderID string) order.Order {
	return GET[order.Order](t, apiURL(), "/orders/"+orderID, nil, http.StatusOK)
}

func getDisputes(t *testing.T) []dispute.Dispute {
	return GET[[]dispute.Dispute](t, apiURL(), "/disputes", nil, http.StatusOK)
}

func getDisputeEvents(t *testing.T, q dispute.DisputeEventQuery) dispute.DisputeEventPage {
	return GET[dispute.DisputeEventPage](t, apiURL(), "/disputes/events", q, http.StatusOK)
}

func getOrderEvents(t *testing.T, q order.OrderEventQuery) order.OrderEventPage {
	return GET[order.OrderEventPage](t, apiURL(), "/orders/events", q, http.StatusOK)
}

func getEvidence(t *testing.T, disputeID string) *dispute.Evidence {
	evidence := GET[dispute.Evidence](t, apiURL(), "/disputes/"+disputeID+"/evidence", nil, http.StatusOK)
	return &evidence
}

func addEvidence(t *testing.T, disputeID string, evidenceData map[string]interface{}) dispute.Evidence {
	return POST[dispute.Evidence](t, apiURL(), "/disputes/"+disputeID+"/evidence", evidenceData, http.StatusOK)
}

func submitDispute(t *testing.T, disputeID string) {
	POST[any](t, apiURL(), "/disputes/"+disputeID+"/submit", nil, http.StatusAccepted)
}

// --- Wait helpers for eventual consistency ---

func waitForOrder(t *testing.T, orderID string, maxRetries int) *order.Order {
	t.Helper()

	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(apiURL() + "/orders/" + orderID)
		require.NoError(t, err)

		if resp.StatusCode == http.StatusOK {
			var o order.Order
			err = json.NewDecoder(resp.Body).Decode(&o)
			resp.Body.Close()
			require.NoError(t, err)
			return &o
		}
		resp.Body.Close()

		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

func waitForOrderStatus(t *testing.T, orderID, expectedStatus string, maxRetries int) *order.Order {
	t.Helper()

	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(apiURL() + "/orders/" + orderID)
		require.NoError(t, err)

		if resp.StatusCode == http.StatusOK {
			var o order.Order
			err = json.NewDecoder(resp.Body).Decode(&o)
			resp.Body.Close()
			require.NoError(t, err)
			if string(o.Status) == expectedStatus {
				return &o
			}
		} else {
			resp.Body.Close()
		}

		time.Sleep(200 * time.Millisecond)
	}

	// Return last state even if status doesn't match
	return waitForOrder(t, orderID, 1)
}

func findDisputeByOrderID(t *testing.T, orderID string) *dispute.Dispute {
	t.Helper()
	disputes := getDisputes(t)
	for _, d := range disputes {
		if d.OrderID == orderID {
			return &d
		}
	}
	return nil
}

func waitForDisputeByOrderID(t *testing.T, orderID string, maxRetries int) *dispute.Dispute {
	t.Helper()

	for i := 0; i < maxRetries; i++ {
		d := findDisputeByOrderID(t, orderID)
		if d != nil {
			return d
		}
		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

func waitForDisputeStatus(t *testing.T, orderID, expectedStatus string, maxRetries int) *dispute.Dispute {
	t.Helper()

	for i := 0; i < maxRetries; i++ {
		d := findDisputeByOrderID(t, orderID)
		if d != nil && string(d.Status) == expectedStatus {
			return d
		}
		time.Sleep(200 * time.Millisecond)
	}

	return findDisputeByOrderID(t, orderID)
}

// --- Composite helpers ---

func createOrderWithId(t *testing.T, orderId string) {
	t.Helper()
	createOrderPayload := map[string]interface{}{
		"provider_event_id": "evt-" + orderId,
		"order_id":          orderId,
		"user_id":           "55555555-5555-5555-5555-555555555555",
		"status":            "created",
		"updated_at":        time.Now().Format(time.RFC3339),
		"created_at":        time.Now().Format(time.RFC3339),
		"meta": map[string]string{
			"amount":   "250.75",
			"currency": "USD",
		},
	}

	sendOrderWebhook(t, createOrderPayload)

	if waitForOrder(t, orderId, 40) == nil {
		t.Fatalf("Order %s was not created in time", orderId)
	}
}

func createOpenedDisputeForOrderId(t *testing.T, orderId string) (disputeId string) {
	t.Helper()
	createOrderWithId(t, orderId)

	openChargeback := map[string]interface{}{
		"provider_event_id": "evt-evidence-1",
		"order_id":          orderId,
		"user_id":           "55555555-5555-5555-5555-555555555555",
		"status":            "opened",
		"reason":            "unauthorized",
		"amount":            250.75,
		"currency":          "USD",
		"occurred_at":       time.Now().Format(time.RFC3339),
		"meta":              map[string]string{},
	}

	sendChargebackWebhook(t, openChargeback)

	foundDispute := waitForDisputeByOrderID(t, orderId, 15)
	if foundDispute == nil {
		t.Fatalf("Could not find dispute for order_id: %s", orderId)
	}

	if foundDispute.Status != "open" {
		t.Errorf("Expected initial dispute status to be 'open', got %v", foundDispute.Status)
	}
	return foundDispute.ID
}
