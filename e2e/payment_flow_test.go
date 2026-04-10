//go:build integration

package integration_test

import (
	"TestTaskJustPay/services/paymanager/domain/payment"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestE2E_PaymentFlow(t *testing.T) {
	if suite.Silvergate == nil {
		t.Skip("Silvergate not configured in test suite")
	}

	truncateDB(t)

	const iterations = 12

	type result struct {
		Num      int
		Amount   int64
		Status   string
		Decline  string
		Final    string
		Duration time.Duration
	}

	results := make([]result, 0, iterations)
	var authorizedPaymentIDs []string

	for i := range iterations {
		amount := int64((i + 1) * 1000)
		start := time.Now()

		resp := POST[payment.Payment](t, apiURL(), "/api/v1/payments",
			payment.CreatePaymentRequest{
				Amount:    amount,
				Currency:  "USD",
				CardToken: fmt.Sprintf("tok_%03d", i+1),
			}, http.StatusOK)

		r := result{
			Num:      i + 1,
			Amount:   amount,
			Status:   string(resp.Status),
			Decline:  resp.DeclineReason,
			Duration: time.Since(start),
		}

		if resp.Status == payment.StatusCapturePending || resp.Status == payment.StatusAuthorized {
			authorizedPaymentIDs = append(authorizedPaymentIDs, resp.ID)
		}

		results = append(results, r)
	}

	// Wait for capture webhooks to propagate through Ingest → Kafka → Paymanager
	for _, id := range authorizedPaymentIDs {
		p := waitForPaymentFinalStatus(t, id, 30)
		if p != nil {
			for i, r := range results {
				if r.Amount == p.Amount {
					results[i].Final = string(p.Status)
					break
				}
			}
		}
	}

	for i, r := range results {
		if r.Status == string(payment.StatusDeclined) {
			results[i].Final = "declined"
		}
	}

	// --- Print ---
	var authorized, declined, captured, capFailed int

	fmt.Println()
	fmt.Println("╔═════╦════════╦══════════════════╦═══════════════════╦══════════════════╦══════════╗")
	fmt.Println("║  #  ║ Amount ║ Auth             ║ Decline Reason    ║ Final            ║ Latency  ║")
	fmt.Println("╠═════╬════════╬══════════════════╬═══════════════════╬══════════════════╬══════════╣")

	for _, r := range results {
		reason := r.Decline
		if reason == "" {
			reason = "-"
		}
		final := r.Final
		if final == "" {
			final = "pending..."
		}
		amountStr := fmt.Sprintf("$%.2f", float64(r.Amount)/100)

		fmt.Printf("║ %3d ║ %6s ║ %-16s ║ %-17s ║ %-16s ║ %7s  ║\n",
			r.Num, amountStr, r.Status, reason, final, r.Duration.Truncate(time.Millisecond))

		switch {
		case r.Status == string(payment.StatusCapturePending) || r.Status == string(payment.StatusAuthorized):
			authorized++
		case r.Status == string(payment.StatusDeclined):
			declined++
		}
		switch r.Final {
		case string(payment.StatusCaptured):
			captured++
		case string(payment.StatusCaptureFailed):
			capFailed++
		}
	}

	fmt.Println("╚═════╩════════╩══════════════════╩═══════════════════╩══════════════════╩══════════╝")
	fmt.Println()
	fmt.Printf("  Auth:    %d authorized, %d declined\n", authorized, declined)
	if authorized > 0 {
		fmt.Printf("  Settle:  %d captured, %d failed\n", captured, capFailed)
	}
	fmt.Println()

	require.Equal(t, iterations, authorized+declined, "all payments should have auth result")
	require.Equal(t, authorized, captured+capFailed, "every authorized payment should reach final status")
}

// waitForPaymentFinalStatus polls GET /api/v1/payments/:id until status is captured or capture_failed.
func waitForPaymentFinalStatus(t *testing.T, paymentID string, maxRetries int) *payment.Payment {
	t.Helper()

	for i := range maxRetries {
		resp, err := http.Get(apiURL() + "/api/v1/payments/" + paymentID)
		require.NoError(t, err)

		if resp.StatusCode == http.StatusOK {
			var p payment.Payment
			err = json.NewDecoder(resp.Body).Decode(&p)
			resp.Body.Close()
			require.NoError(t, err)

			if p.Status == payment.StatusCaptured || p.Status == payment.StatusCaptureFailed {
				return &p
			}
		} else {
			resp.Body.Close()
		}

		if i < maxRetries-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Return last state
	resp, err := http.Get(apiURL() + "/api/v1/payments/" + paymentID)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var p payment.Payment
		_ = json.NewDecoder(resp.Body).Decode(&p)
		return &p
	}
	return nil
}
