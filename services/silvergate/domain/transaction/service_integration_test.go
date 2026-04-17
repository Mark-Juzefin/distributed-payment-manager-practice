//go:build integration

package transaction_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/pkg/testinfra"
	silvergate "TestTaskJustPay/services/silvergate"
	"TestTaskJustPay/services/silvergate/acquirer"
	"TestTaskJustPay/services/silvergate/domain/transaction"
	txrepo "TestTaskJustPay/services/silvergate/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *postgres.Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()
	pgContainer, err := testinfra.NewPostgresWithConfig(ctx, testinfra.PostgresConfig{
		DBName:      "silvergate_race_test",
		MigrationFS: silvergate.MigrationFS(),
		Image:       "postgres:17",
	})
	if err != nil {
		panic(fmt.Sprintf("postgres: %v", err))
	}
	pg = pgContainer.Pool
	code := m.Run()
	pgContainer.Cleanup(ctx)
	os.Exit(code)
}

// stubWebhooks tracks async completions via channels.
type stubWebhooks struct {
	captureDone chan struct{}
	refundDone  chan struct{}
}

func newStubWebhooks() *stubWebhooks {
	return &stubWebhooks{
		captureDone: make(chan struct{}, 10),
		refundDone:  make(chan struct{}, 10),
	}
}

func (w *stubWebhooks) SendCaptureResult(_ context.Context, _ *transaction.Transaction) error {
	w.captureDone <- struct{}{}
	return nil
}

func (w *stubWebhooks) SendRefundResult(_ context.Context, _ *transaction.Transaction, _ *transaction.Refund) error {
	w.refundDone <- struct{}{}
	return nil
}

func (w *stubWebhooks) waitCaptures(n int, t *testing.T) {
	t.Helper()
	for range n {
		select {
		case <-w.captureDone:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for capture webhook")
		}
	}
}

func (w *stubWebhooks) waitRefunds(n int, t *testing.T) {
	t.Helper()
	for range n {
		select {
		case <-w.refundDone:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for refund webhook")
		}
	}
}

// TestConcurrentRefund_Overdraft reproduces the lost-update race condition
// in Service.Refund (service.go:130). Three concurrent refunds on a $50
// payment all read refunded_amount=0 and pass validation simultaneously.
//
// Expected: at most 1 refund accepted, total refunded ≤ $50.
// Actual (bug): all 3 accepted, total refunded = $115.
func TestConcurrentRefund_Overdraft(t *testing.T) {
	ctx := context.Background()

	repo := txrepo.NewPgTransactionRepo(pg.Pool)
	acq := acquirer.NewMockAcquirer(1.0, 1.0, 50*time.Millisecond)
	wh := newStubWebhooks()
	txRepoFactory := func(tx postgres.Executor) transaction.Repo {
		return txrepo.NewPgTransactionRepo(tx)
	}
	svc := transaction.NewService(repo, acq, wh, slog.Default(), pg, txRepoFactory)

	// --- Setup: auth + capture a $50 transaction ---

	auth, err := svc.Authorize(ctx, transaction.AuthRequest{
		MerchantID: "merchant_race",
		OrderID:    fmt.Sprintf("race_%d", time.Now().UnixNano()),
		Amount:     5000,
		Currency:   "USD",
		CardToken:  "tok_race",
	})
	require.NoError(t, err)
	require.Equal(t, transaction.StatusAuthorized, auth.Status)

	_, err = svc.Capture(ctx, transaction.CaptureRequest{
		TransactionID:  auth.TransactionID,
		Amount:         5000,
		IdempotencyKey: fmt.Sprintf("cap_%d", time.Now().UnixNano()),
	})
	require.NoError(t, err)

	// Poll until async settle completes
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tx, _ := repo.GetByID(ctx, auth.TransactionID)
		if tx != nil && tx.Status == transaction.StatusCaptured {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	tx, err := repo.GetByID(ctx, auth.TransactionID)
	require.NoError(t, err)
	require.Equal(t, transaction.StatusCaptured, tx.Status)

	// --- 3 concurrent refunds: $30 + $40 + $45 on a $50 payment ---

	amounts := []int64{3000, 4000, 4500}
	errs := make([]error, len(amounts))

	var wg sync.WaitGroup
	wg.Add(len(amounts))
	for i, amt := range amounts {
		go func(idx int, amount int64) {
			defer wg.Done()
			_, errs[idx] = svc.Refund(ctx, transaction.RefundRequest{
				TransactionID:  auth.TransactionID,
				Amount:         amount,
				IdempotencyKey: fmt.Sprintf("ref_%d_%d", idx, time.Now().UnixNano()),
			})
		}(i, amt)
	}
	wg.Wait()

	// Count how many passed validation
	var accepted int
	for _, e := range errs {
		if e == nil {
			accepted++
		}
	}
	t.Logf("validation: %d/%d refunds accepted", accepted, len(amounts))

	// Wait for async processing of accepted refunds
	wh.waitRefunds(accepted, t)

	// Query actual total refunded from refund records (source of truth)
	var totalRefunded int64
	err = pg.Pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(amount), 0) FROM refunds WHERE transaction_id = $1 AND status = 'refunded'",
		auth.TransactionID,
	).Scan(&totalRefunded)
	require.NoError(t, err)

	t.Logf("result: accepted=%d, total_refunded=%d, tx_amount=%d", accepted, totalRefunded, tx.Amount)

	// BUG: all 3 pass validation because each reads refunded_amount=0
	assert.Less(t, accepted, len(amounts),
		"not all refunds should pass: $30+$40+$45 exceeds $50")

	// BUG: sum of successful refund records exceeds payment amount
	assert.LessOrEqual(t, totalRefunded, tx.Amount,
		"total refunded (%d) must not exceed payment (%d)", totalRefunded, tx.Amount)
}

// TestConcurrentCapture_DuplicateSettle reproduces a lost-update race in
// Service.Capture. Two concurrent captures on the same authorized transaction
// both read status=authorized, both mark capture_pending, both launch settleAsync.
//
// Expected: exactly 1 capture succeeds.
// Actual (bug): both succeed, two settleAsync goroutines run.
func TestConcurrentCapture_DuplicateSettle(t *testing.T) {
	ctx := context.Background()

	repo := txrepo.NewPgTransactionRepo(pg.Pool)
	acq := acquirer.NewMockAcquirer(1.0, 1.0, 50*time.Millisecond)
	wh := newStubWebhooks()
	txRepoFactory := func(tx postgres.Executor) transaction.Repo {
		return txrepo.NewPgTransactionRepo(tx)
	}
	svc := transaction.NewService(repo, acq, wh, slog.Default(), pg, txRepoFactory)

	// Auth a $100 transaction
	auth, err := svc.Authorize(ctx, transaction.AuthRequest{
		MerchantID: "merchant_cap_race",
		OrderID:    fmt.Sprintf("cap_race_%d", time.Now().UnixNano()),
		Amount:     10000,
		Currency:   "USD",
		CardToken:  "tok_cap_race",
	})
	require.NoError(t, err)
	require.Equal(t, transaction.StatusAuthorized, auth.Status)

	// 3 concurrent captures on the same authorized transaction
	const n = 3
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.Capture(ctx, transaction.CaptureRequest{
				TransactionID:  auth.TransactionID,
				Amount:         10000,
				IdempotencyKey: fmt.Sprintf("cap_%d_%d", idx, time.Now().UnixNano()),
			})
		}(i)
	}
	wg.Wait()

	var accepted int
	for _, e := range errs {
		if e == nil {
			accepted++
		}
	}
	t.Logf("capture validation: %d/%d accepted", accepted, n)

	// Wait for webhooks from accepted captures
	wh.waitCaptures(accepted, t)

	// Exactly 1 capture should succeed
	assert.Equal(t, 1, accepted,
		"exactly one concurrent capture should succeed, got %d", accepted)
}

// TestSettleAsync_BlindUpdate proves that settleAsync overwrites status changes
// made by other operations while the acquirer call is in progress.
//
// Scenario:
//  1. Capture → status = capture_pending, settleAsync starts (acquirer blocks 200ms)
//  2. While blocked — we UPDATE status to 'voided' directly in DB (simulating a concurrent op)
//  3. settleAsync returns — blind UPDATE overwrites 'voided' with 'captured'
//
// Expected: status stays 'voided' (settleAsync should detect the change).
// Actual (bug): status becomes 'captured' — blind update ignores concurrent change.
func TestSettleAsync_BlindUpdate(t *testing.T) {
	ctx := context.Background()

	repo := txrepo.NewPgTransactionRepo(pg.Pool)
	// 200ms settle delay — gives us a window to modify DB while settleAsync waits
	acq := acquirer.NewMockAcquirer(1.0, 1.0, 200*time.Millisecond)
	wh := newStubWebhooks()
	txRepoFactory := func(tx postgres.Executor) transaction.Repo {
		return txrepo.NewPgTransactionRepo(tx)
	}
	svc := transaction.NewService(repo, acq, wh, slog.Default(), pg, txRepoFactory)

	// Auth
	auth, err := svc.Authorize(ctx, transaction.AuthRequest{
		MerchantID: "merchant_settle_race",
		OrderID:    fmt.Sprintf("settle_race_%d", time.Now().UnixNano()),
		Amount:     5000,
		Currency:   "USD",
		CardToken:  "tok_settle_race",
	})
	require.NoError(t, err)

	// Capture — starts settleAsync which blocks on acquirer for 200ms
	_, err = svc.Capture(ctx, transaction.CaptureRequest{
		TransactionID:  auth.TransactionID,
		Amount:         5000,
		IdempotencyKey: fmt.Sprintf("cap_settle_%d", time.Now().UnixNano()),
	})
	require.NoError(t, err)

	// Simulate a concurrent operation changing status while settleAsync is blocked
	_, err = pg.Pool.Exec(ctx,
		"UPDATE transactions SET status = 'voided' WHERE id = $1",
		auth.TransactionID)
	require.NoError(t, err)

	// Wait for settleAsync to finish (200ms acquirer + some buffer)
	time.Sleep(400 * time.Millisecond)

	// Check final status
	tx, err := repo.GetByID(ctx, auth.TransactionID)
	require.NoError(t, err)

	t.Logf("final status: %s (expected: voided)", tx.Status)

	// BUG: settleAsync does blind UPDATE, overwrites 'voided' with 'captured'
	assert.Equal(t, transaction.StatusVoided, tx.Status,
		"settleAsync should not overwrite status changed by another operation")
}

// TestVoidVsCapture_Race proves that without SELECT FOR UPDATE, a concurrent
// Capture can slip through while Void is calling the acquirer.
//
// Scenario:
//  1. Void and Capture start concurrently on the same authorized transaction
//  2. Without FOR UPDATE, both read status=authorized and proceed
//  3. Both succeed — void calls acquirer AND capture calls acquirer
//
// Expected: exactly one succeeds, the other is rejected.
func TestVoidVsCapture_Race(t *testing.T) {
	ctx := context.Background()

	repo := txrepo.NewPgTransactionRepo(pg.Pool)
	// Slow void (200ms), fast settle (50ms) — Void holds lock while calling acquirer
	acq := acquirer.NewMockAcquirer(1.0, 1.0, 50*time.Millisecond)
	wh := newStubWebhooks()
	txRepoFactory := func(tx postgres.Executor) transaction.Repo {
		return txrepo.NewPgTransactionRepo(tx)
	}
	svc := transaction.NewService(repo, acq, wh, slog.Default(), pg, txRepoFactory)

	auth, err := svc.Authorize(ctx, transaction.AuthRequest{
		MerchantID: "merchant_void_cap",
		OrderID:    fmt.Sprintf("void_cap_%d", time.Now().UnixNano()),
		Amount:     5000,
		Currency:   "USD",
		CardToken:  "tok_void_cap",
	})
	require.NoError(t, err)

	// Fire Void and Capture concurrently
	var voidErr, captureErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, voidErr = svc.Void(ctx, auth.TransactionID)
	}()
	go func() {
		defer wg.Done()
		_, captureErr = svc.Capture(ctx, transaction.CaptureRequest{
			TransactionID:  auth.TransactionID,
			Amount:         5000,
			IdempotencyKey: fmt.Sprintf("cap_void_%d", time.Now().UnixNano()),
		})
	}()
	wg.Wait()

	t.Logf("void err: %v, capture err: %v", voidErr, captureErr)

	// Exactly one should succeed
	voidOK := voidErr == nil
	captureOK := captureErr == nil
	assert.True(t, voidOK != captureOK,
		"exactly one should succeed: void=%v, capture=%v", voidOK, captureOK)
}
