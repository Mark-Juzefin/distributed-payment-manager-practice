package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Order statuses (mirrors internal/api/domain/order)
const (
	orderCreated = "created"
	orderUpdated = "updated"
	orderSuccess = "success"
	orderFailed  = "failed"
)

// orderTransitions defines valid next statuses from each status.
var orderTransitions = map[string][]string{
	orderCreated: {orderUpdated, orderSuccess, orderFailed},
	orderUpdated: {orderUpdated, orderSuccess, orderFailed},
}

type Result struct {
	Endpoint string
	Duration time.Duration
	Status   int
	Error    error
}

type Stats struct {
	Total     atomic.Int64
	Success   atomic.Int64
	Errors    atomic.Int64
	Orders    atomic.Int64
	Disputes  atomic.Int64
	Latencies []time.Duration
	mu        sync.Mutex
}

func (s *Stats) Record(r Result) {
	s.Total.Add(1)
	if r.Error != nil || r.Status >= 400 {
		s.Errors.Add(1)
	} else {
		s.Success.Add(1)
	}
	if r.Endpoint == "/webhooks/payments/orders" {
		s.Orders.Add(1)
	} else {
		s.Disputes.Add(1)
	}
	s.mu.Lock()
	s.Latencies = append(s.Latencies, r.Duration)
	s.mu.Unlock()
}

var disputeRatio = flag.Float64("dispute-ratio", 0.3, "Probability of dispute per successful order")

func main() {
	target := flag.String("target", "http://localhost:3001", "Ingest service URL")
	vus := flag.Int("vus", 10, "Virtual users (concurrent workers)")
	duration := flag.Duration("duration", 0, "Test duration (0 = run until Ctrl+C)")
	flag.Parse()

	var ctx context.Context
	var cancel context.CancelFunc
	if *duration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), *duration)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	// Handle Ctrl+C
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		fmt.Println("\nStopping...")
		cancel()
	}()

	stats := &Stats{}
	var wg sync.WaitGroup

	durationStr := "until Ctrl+C"
	if *duration > 0 {
		durationStr = duration.String()
	}
	fmt.Printf("Starting load test: %d VUs, %s\n", *vus, durationStr)
	fmt.Printf("Target: %s\n", *target)
	fmt.Printf("Dispute ratio: %.0f%%\n\n", *disputeRatio*100)

	start := time.Now()
	for i := 0; i < *vus; i++ {
		wg.Add(1)
		go runVU(ctx, &wg, *target, stats)
	}

	wg.Wait()
	printSummary(stats, time.Since(start))
}

func runVU(ctx context.Context, wg *sync.WaitGroup, target string, stats *Stats) {
	defer wg.Done()
	client := &http.Client{Timeout: 5 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			orderID, userID, finalStatus := runOrderScenario(ctx, client, target, stats)

			if finalStatus == orderSuccess && rand.Float64() < *disputeRatio {
				runDisputeScenario(ctx, client, target, orderID, userID, stats)
			}
		}
	}
}

func runOrderScenario(ctx context.Context, client *http.Client, target string, stats *Stats) (string, string, string) {
	orderID := uuid.NewString()
	userID := uuid.NewString()
	now := time.Now()

	// Step 1: always start with "created"
	result := sendOrderWebhook(client, target, orderID, userID, orderCreated, now)
	stats.Record(result)
	if result.Error != nil || result.Status >= 400 {
		return orderID, userID, orderCreated
	}

	currentStatus := orderCreated

	// Step 2: walk through transitions until terminal state
	for {
		select {
		case <-ctx.Done():
			return orderID, userID, currentStatus
		case <-time.After(10 * time.Millisecond):
		}

		next := pickNextStatus(currentStatus)
		if next == "" {
			return orderID, userID, currentStatus
		}

		now = time.Now()
		result = sendOrderWebhook(client, target, orderID, userID, next, now)
		stats.Record(result)
		if result.Error != nil || result.Status >= 400 {
			return orderID, userID, currentStatus
		}

		currentStatus = next
		if currentStatus == orderSuccess || currentStatus == orderFailed {
			return orderID, userID, currentStatus
		}
	}
}

func pickNextStatus(current string) string {
	candidates := orderTransitions[current]
	if len(candidates) == 0 {
		return ""
	}
	return candidates[rand.Intn(len(candidates))]
}

func runDisputeScenario(ctx context.Context, client *http.Client, target, orderID, userID string, stats *Stats) {
	now := time.Now()
	evidenceDue := now.Add(14 * 24 * time.Hour)

	// 1. Chargeback opened
	result := sendChargebackWebhook(client, target, orderID, userID, "opened", "", now, &evidenceDue)
	stats.Record(result)
	if result.Error != nil || result.Status >= 400 {
		return
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(10 * time.Millisecond):
	}

	// 2. Maybe "updated" (evidence window change) — 50% chance
	if rand.Float32() < 0.5 {
		now = time.Now()
		newDeadline := now.Add(7 * 24 * time.Hour)
		result = sendChargebackWebhook(client, target, orderID, userID, "updated", "", now, &newDeadline)
		stats.Record(result)
		if result.Error != nil || result.Status >= 400 {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
		}
	}

	// 3. Closed with resolution (won/lost)
	resolution := "won"
	if rand.Float32() < 0.5 {
		resolution = "lost"
	}
	now = time.Now()
	result = sendChargebackWebhook(client, target, orderID, userID, "closed", resolution, now, nil)
	stats.Record(result)
}

func sendOrderWebhook(client *http.Client, target, orderID, userID, status string, now time.Time) Result {
	payload := map[string]any{
		"provider_event_id": uuid.NewString(),
		"order_id":          orderID,
		"user_id":           userID,
		"status":            status,
		"updated_at":        now.Format(time.RFC3339),
		"created_at":        now.Format(time.RFC3339),
	}
	return sendWebhook(client, target, "/webhooks/payments/orders", payload)
}

func sendChargebackWebhook(client *http.Client, target, orderID, userID, status, resolution string, now time.Time, evidenceDue *time.Time) Result {
	payload := map[string]any{
		"provider_event_id": uuid.NewString(),
		"order_id":          orderID,
		"user_id":           userID,
		"status":            status,
		"reason":            "fraudulent",
		"amount":            99.99,
		"currency":          "USD",
		"occurred_at":       now.Format(time.RFC3339),
	}
	if evidenceDue != nil {
		payload["evidence_due_at"] = evidenceDue.Format(time.RFC3339)
	}
	if resolution != "" {
		payload["meta"] = map[string]string{"resolution": resolution}
	}
	return sendWebhook(client, target, "/webhooks/payments/chargebacks", payload)
}

func sendWebhook(client *http.Client, target, endpoint string, payload map[string]any) Result {
	body, _ := json.Marshal(payload)
	url := target + endpoint

	start := time.Now()
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	duration := time.Since(start)

	result := Result{Endpoint: endpoint, Duration: duration, Error: err}
	if resp != nil {
		result.Status = resp.StatusCode
		resp.Body.Close()
	}
	return result
}

func printSummary(stats *Stats, elapsed time.Duration) {
	total := stats.Total.Load()
	success := stats.Success.Load()
	errors := stats.Errors.Load()
	orders := stats.Orders.Load()
	disputes := stats.Disputes.Load()

	fmt.Println("\n========== SUMMARY ==========")
	fmt.Printf("Duration:    %s\n", elapsed.Round(time.Millisecond))
	if total > 0 {
		fmt.Printf("Requests:    %d (%.1f/s)\n", total, float64(total)/elapsed.Seconds())
	} else {
		fmt.Printf("Requests:    0\n")
	}
	fmt.Printf("  Orders:    %d\n", orders)
	fmt.Printf("  Disputes:  %d\n", disputes)
	fmt.Printf("Success:     %d\n", success)
	if total > 0 {
		fmt.Printf("Errors:      %d (%.2f%%)\n", errors, float64(errors)/float64(total)*100)
	} else {
		fmt.Printf("Errors:      0\n")
	}

	stats.mu.Lock()
	latencies := stats.Latencies
	stats.mu.Unlock()

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50 := latencies[len(latencies)*50/100]
		p95 := latencies[len(latencies)*95/100]
		p99 := latencies[len(latencies)*99/100]
		fmt.Printf("Latency:     p50=%s  p95=%s  p99=%s\n", p50, p95, p99)
	}
	fmt.Println("==============================")
}
