# Plan: Subtask 6 — Go-based Load Testing

## Goal

Simple load test that generates realistic webhook sequences using domain models directly.

## Structure

```
loadtest/
└── main.go   # Everything in one file (~150-200 lines)
```

## Implementation

```go
// loadtest/main.go
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
    "github.com/illia/distributed-payment-manager-practice/internal/api/domain/order"
)

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

func main() {
    target := flag.String("target", "http://localhost:3001", "Ingest service URL")
    vus := flag.Int("vus", 10, "Virtual users")
    duration := flag.Duration("duration", 30*time.Second, "Test duration")
    flag.Parse()

    ctx, cancel := context.WithTimeout(context.Background(), *duration)
    defer cancel()

    // Handle Ctrl+C
    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, os.Interrupt)
        <-sigCh
        cancel()
    }()

    stats := &Stats{}
    var wg sync.WaitGroup

    fmt.Printf("Starting load test: %d VUs, %s duration\n", *vus, *duration)
    fmt.Printf("Target: %s\n\n", *target)

    start := time.Now()
    for i := 0; i < *vus; i++ {
        wg.Add(1)
        go runVU(ctx, &wg, *target, stats)
    }

    wg.Wait()
    printSummary(stats, time.Since(start))
}

var disputeRatio = flag.Float64("dispute-ratio", 0.3, "Probability of dispute per successful order")

func runVU(ctx context.Context, wg *sync.WaitGroup, target string, stats *Stats) {
    defer wg.Done()
    client := &http.Client{Timeout: 5 * time.Second}

    for {
        select {
        case <-ctx.Done():
            return
        default:
            orderID, finalStatus := runOrderScenario(ctx, client, target, stats)

            // Create dispute for some successful orders
            if finalStatus == order.StatusSuccess && rand.Float64() < *disputeRatio {
                runDisputeScenario(ctx, client, target, orderID, stats)
            }
        }
    }
}

func runOrderScenario(ctx context.Context, client *http.Client, target string, stats *Stats) (string, order.Status) {
    orderID := uuid.NewString()
    userID := uuid.NewString()
    currentStatus := order.StatusCreated

    for {
        result := sendOrderWebhook(client, target, orderID, userID, currentStatus)
        stats.Record(result)

        if result.Error != nil {
            return orderID, currentStatus
        }

        // Terminal state reached
        if currentStatus == order.StatusSuccess || currentStatus == order.StatusFailed {
            return orderID, currentStatus
        }

        // Get next valid status using domain logic
        nextStatus := pickNextStatus(currentStatus)
        if nextStatus == "" {
            return orderID, currentStatus
        }
        currentStatus = nextStatus

        select {
        case <-ctx.Done():
            return orderID, currentStatus
        case <-time.After(10 * time.Millisecond):
        }
    }
}

func runDisputeScenario(ctx context.Context, client *http.Client, target, orderID string, stats *Stats) {
    disputeID := uuid.NewString()

    // 1. Chargeback opened
    result := sendChargebackWebhook(client, target, disputeID, orderID, "opened", "")
    stats.Record(result)
    if result.Error != nil {
        return
    }

    select {
    case <-ctx.Done():
        return
    case <-time.After(10 * time.Millisecond):
    }

    // 2. Maybe updated (evidence window) - 50% chance
    if rand.Float32() < 0.5 {
        result = sendChargebackWebhook(client, target, disputeID, orderID, "updated", "")
        stats.Record(result)
        if result.Error != nil {
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
    result = sendChargebackWebhook(client, target, disputeID, orderID, "closed", resolution)
    stats.Record(result)
}

// pickNextStatus uses domain logic to pick valid next status
func pickNextStatus(current order.Status) order.Status {
    candidates := []order.Status{order.StatusUpdated, order.StatusSuccess, order.StatusFailed}

    for _, next := range candidates {
        if current.CanBeUpdatedTo(next) {
            // Simple selection: prefer terminal states to keep scenarios short
            return next
        }
    }
    return ""
}

func sendOrderWebhook(client *http.Client, target, orderID, userID string, status order.Status) Result {
    payload := map[string]any{
        "provider_order_id": orderID,
        "status":            status,
        "user_id":           userID,
        "amount":            99.99,
        "currency":          "USD",
        "occurred_at":       time.Now().Format(time.RFC3339),
        "event_id":          uuid.NewString(),
    }
    return sendWebhook(client, target, "/webhooks/payments/orders", payload)
}

func sendChargebackWebhook(client *http.Client, target, disputeID, orderID, status, resolution string) Result {
    payload := map[string]any{
        "chargeback_id":     disputeID,
        "provider_order_id": orderID,
        "status":            status,
        "reason":            "fraudulent",
        "amount":            99.99,
        "currency":          "USD",
        "occurred_at":       time.Now().Format(time.RFC3339),
        "event_id":          uuid.NewString(),
    }
    if resolution != "" {
        payload["meta"] = map[string]any{"resolution": resolution}
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
    fmt.Printf("Requests:    %d (%.1f/s)\n", total, float64(total)/elapsed.Seconds())
    fmt.Printf("  Orders:    %d\n", orders)
    fmt.Printf("  Disputes:  %d\n", disputes)
    fmt.Printf("Success:     %d\n", success)
    fmt.Printf("Errors:      %d (%.2f%%)\n", errors, float64(errors)/float64(total)*100)

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
```

## Makefile Targets

```makefile
.PHONY: loadtest loadtest-http loadtest-kafka

loadtest:
	go run ./loadtest -target http://localhost:3001 -vus 10 -duration 30s

loadtest-http: run-http
	go run ./loadtest -target http://localhost:3001 -vus 10 -duration 30s

loadtest-kafka: run-kafka
	go run ./loadtest -target http://localhost:3001 -vus 10 -duration 30s
```

## Implementation Checklist

- [ ] Create `loadtest/main.go` with VU runner and stats
- [ ] Order lifecycle using `order.Status.CanBeUpdatedTo()`
- [ ] Dispute scenario (30% of successful orders, full lifecycle)
- [ ] Makefile targets
- [ ] Test both HTTP and Kafka modes

## Usage

```bash
# Default (10 VUs, 30s)
go run ./loadtest

# Custom
go run ./loadtest -vus 50 -duration 2m -target http://localhost:3001
```

## Expected Output

```
Starting load test: 10 VUs, 30s duration
Target: http://localhost:3001

========== SUMMARY ==========
Duration:    30.001s
Requests:    5765 (192.2/s)
  Orders:    4523
  Disputes:  1242
Success:     5741
Errors:      24 (0.42%)
Latency:     p50=12ms  p95=45ms  p99=120ms
==============================
```
