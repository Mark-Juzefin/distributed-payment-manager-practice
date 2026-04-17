//go:build integration

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"TestTaskJustPay/pkg/testinfra"
	silvergate "TestTaskJustPay/services/silvergate"
	"TestTaskJustPay/services/silvergate/config"

	"github.com/stretchr/testify/require"
)

var (
	silvergateURL string
	webhooks      *webhookCollector
)

// --- webhook collector ---

type webhookCollector struct {
	mu       sync.Mutex
	events   []webhookEvent
	received chan struct{}
	server   *http.Server
}

type webhookEvent struct {
	Event         string `json:"event"`
	TransactionID string `json:"transaction_id"`
	OrderID       string `json:"order_id"`
	MerchantID    string `json:"merchant_id"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Timestamp     string `json:"timestamp"`
}

func startWebhookCollector(listener net.Listener) *webhookCollector {
	wc := &webhookCollector{
		received: make(chan struct{}, 200),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/webhooks/silvergate", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var evt webhookEvent
		_ = json.Unmarshal(body, &evt)
		wc.mu.Lock()
		wc.events = append(wc.events, evt)
		wc.mu.Unlock()
		wc.received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	})
	wc.server = &http.Server{Handler: mux}
	go wc.server.Serve(listener)
	return wc
}

func (wc *webhookCollector) waitFor(n int, timeout time.Duration) {
	for range n {
		select {
		case <-wc.received:
		case <-time.After(timeout):
			return
		}
	}
}

func (wc *webhookCollector) drain() []webhookEvent {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	cp := make([]webhookEvent, len(wc.events))
	copy(cp, wc.events)
	wc.events = wc.events[:0]
	return cp
}

// --- setup ---

func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. PostgreSQL
	pgContainer, err := testinfra.NewPostgresWithConfig(ctx, testinfra.PostgresConfig{
		DBName:      "silvergate_test",
		MigrationFS: silvergate.MigrationFS(),
	})
	if err != nil {
		panic(fmt.Sprintf("postgres: %v", err))
	}

	// 2. Webhook collector (random free port)
	whListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("webhook listener: %v", err))
	}
	whPort := whListener.Addr().(*net.TCPAddr).Port
	webhooks = startWebhookCollector(whListener)

	// 3. Silvergate service (random free port)
	sgListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("silvergate listener: %v", err))
	}
	sgPort := sgListener.Addr().(*net.TCPAddr).Port
	sgListener.Close() // release so the app can bind

	silvergateURL = fmt.Sprintf("http://127.0.0.1:%d", sgPort)

	cfg := config.Config{
		Port:                      sgPort,
		PgURL:                     pgContainer.DSN,
		LogLevel:                  "debug",
		WebhookCallbackURL:        fmt.Sprintf("http://127.0.0.1:%d/webhooks/silvergate", whPort),
		AcquirerAuthApproveRate:   0.85,
		AcquirerSettleSuccessRate: 0.90,
		AcquirerSettleDelay:       100 * time.Millisecond,
	}

	app, err := silvergate.NewApp(cfg)
	if err != nil {
		panic(fmt.Sprintf("silvergate app: %v", err))
	}
	go app.Run()

	// Wait for ready
	for i := range 50 {
		resp, err := http.Get(silvergateURL + "/health/ready")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if i == 49 {
			panic("silvergate not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}

	code := m.Run()

	webhooks.server.Close()
	pgContainer.Cleanup(ctx)
	os.Exit(code)
}

// --- helpers ---

type authRequest struct {
	MerchantID string `json:"merchant_id"`
	OrderID    string `json:"order_id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	CardToken  string `json:"card_token"`
}

type authResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	OrderID       string `json:"order_id"`
	DeclineReason string `json:"decline_reason,omitempty"`
}

type captureRequest struct {
	TransactionID  string `json:"transaction_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

type captureResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	j, err := json.Marshal(body)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(j))
	require.NoError(t, err)
	return resp
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&v))
	return v
}

// --- test ---

func TestAuthCaptureFlow(t *testing.T) {
	const iterations = 15

	type result struct {
		OrderID       string
		Amount        int64
		AuthStatus    string
		DeclineReason string
		CaptureHTTP   int
		WebhookEvent  string
		WebhookStatus string
	}

	results := make([]result, 0, iterations)
	var capturesInitiated int

	for i := range iterations {
		orderID := fmt.Sprintf("ord_%03d", i+1)
		amount := int64((i + 1) * 1000)

		// 1. Auth
		resp := postJSON(t, silvergateURL+"/api/v1/auth", authRequest{
			MerchantID: "merchant_e2e",
			OrderID:    orderID,
			Amount:     amount,
			Currency:   "USD",
			CardToken:  fmt.Sprintf("tok_%03d", i+1),
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		auth := decodeJSON[authResponse](t, resp)

		r := result{
			OrderID:       orderID,
			Amount:        amount,
			AuthStatus:    auth.Status,
			DeclineReason: auth.DeclineReason,
		}

		// 2. Capture if authorized
		if auth.Status == "authorized" {
			resp := postJSON(t, silvergateURL+"/api/v1/capture", captureRequest{
				TransactionID:  auth.TransactionID,
				Amount:         amount,
				IdempotencyKey: fmt.Sprintf("cap_%03d", i+1),
			})
			r.CaptureHTTP = resp.StatusCode
			resp.Body.Close()
			capturesInitiated++
		}

		results = append(results, r)
	}

	// 3. Wait for async webhooks
	webhooks.waitFor(capturesInitiated, 10*time.Second)
	whEvents := webhooks.drain()

	// Match webhooks to orders
	whByOrder := make(map[string]webhookEvent, len(whEvents))
	for _, e := range whEvents {
		whByOrder[e.OrderID] = e
	}
	for i, r := range results {
		if e, ok := whByOrder[r.OrderID]; ok {
			results[i].WebhookEvent = e.Event
			results[i].WebhookStatus = e.Status
		}
	}

	// --- Print ---

	fmt.Println()
	fmt.Println("╔══════════╦════════╦═════════════╦═══════════════════╦═════════╦═══════════════════════════════════════╗")
	fmt.Println("║ Order    ║ Amount ║ Auth        ║ Decline Reason    ║ Capture ║ Webhook                               ║")
	fmt.Println("╠══════════╬════════╬═════════════╬═══════════════════╬═════════╬═══════════════════════════════════════╣")

	var authorized, declined, captured, capFailed int

	for _, r := range results {
		reason := r.DeclineReason
		if reason == "" {
			reason = "-"
		}

		capStr := "-"
		if r.CaptureHTTP != 0 {
			capStr = fmt.Sprintf("%d", r.CaptureHTTP)
		}

		whStr := "-"
		if r.WebhookEvent != "" {
			whStr = fmt.Sprintf("%s → %s", r.WebhookEvent, r.WebhookStatus)
		}

		amountStr := fmt.Sprintf("$%.2f", float64(r.Amount)/100)

		fmt.Printf("║ %-8s ║ %6s ║ %-11s ║ %-17s ║ %7s ║ %-37s ║\n",
			r.OrderID, amountStr, r.AuthStatus, reason, capStr, whStr)

		switch r.AuthStatus {
		case "authorized":
			authorized++
		case "declined":
			declined++
		}
		switch r.WebhookStatus {
		case "captured":
			captured++
		case "capture_failed":
			capFailed++
		}
	}

	fmt.Println("╚══════════╩════════╩═════════════╩═══════════════════╩═════════╩═══════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Auth:      %d authorized, %d declined (%.0f%% approve)\n",
		authorized, declined, pct(authorized, iterations))
	if authorized > 0 {
		fmt.Printf("  Settle:    %d captured, %d failed (%.0f%% success)\n",
			captured, capFailed, pct(captured, authorized))
		fmt.Printf("  Webhooks:  %d/%d received\n", len(whEvents), authorized)
	}
	fmt.Println()

	// Basic assertions
	require.Equal(t, iterations, authorized+declined, "all auths should complete")
	require.Equal(t, authorized, len(whEvents), "every capture should produce a webhook")
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}
