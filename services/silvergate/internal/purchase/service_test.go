package purchase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/transaction"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// --- fakes ---

type fakeProductService struct {
	getCalled    bool
	getMerchant  string
	getID        uuid.UUID
	getResp      *product.Product
	getErr       error
	markCalled   bool
	markMerchant string
	markID       uuid.UUID
	markErr      error
}

func (f *fakeProductService) Get(_ context.Context, merchantID string, id uuid.UUID) (*product.Product, error) {
	f.getCalled = true
	f.getMerchant = merchantID
	f.getID = id
	return f.getResp, f.getErr
}

func (f *fakeProductService) MarkPurchasedInTx(_ context.Context, _ postgres.Executor, merchantID string, id uuid.UUID) error {
	f.markCalled = true
	f.markMerchant = merchantID
	f.markID = id
	return f.markErr
}

type fakeAuthorizer struct {
	called   bool
	gotReq   transaction.AuthRequest
	respTx   *transaction.Transaction
	respErr  error
	approved bool
}

func (f *fakeAuthorizer) AuthorizeInTx(_ context.Context, _ transaction.Repo, req transaction.AuthRequest) (*transaction.Transaction, error) {
	f.called = true
	f.gotReq = req
	return f.respTx, f.respErr
}

type fakeCapturer struct {
	called bool
	gotReq transaction.CaptureRequest
	resp   transaction.CaptureResponse
	err    error
}

func (f *fakeCapturer) Capture(_ context.Context, req transaction.CaptureRequest) (transaction.CaptureResponse, error) {
	f.called = true
	f.gotReq = req
	return f.resp, f.err
}

type fakeTxLookup struct {
	calls   int
	resp    *transaction.Transaction
	err     error
	respSeq []lookupResult
}
type lookupResult struct {
	tx  *transaction.Transaction
	err error
}

func (f *fakeTxLookup) GetByPurchaseIdempotencyKey(_ context.Context, _, _ string) (*transaction.Transaction, error) {
	idx := f.calls
	f.calls++
	if idx < len(f.respSeq) {
		return f.respSeq[idx].tx, f.respSeq[idx].err
	}
	return f.resp, f.err
}

type fakeTransactor struct {
	callbackErr error
}

func (f fakeTransactor) InTransaction(_ context.Context, _ pgx.TxIsoLevel, fn func(postgres.Executor) error) error {
	return fn(nil)
}

func newServiceWithFakes(t *testing.T) (
	*Service,
	*fakeProductService,
	*fakeAuthorizer,
	*fakeCapturer,
	*fakeTxLookup,
) {
	t.Helper()
	products := &fakeProductService{}
	authorizer := &fakeAuthorizer{}
	capturer := &fakeCapturer{}
	lookup := &fakeTxLookup{err: transaction.ErrNotFound}
	svc := NewService(
		products, authorizer, capturer, lookup,
		func(postgres.Executor) transaction.Repo { return nil },
		fakeTransactor{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return svc, products, authorizer, capturer, lookup
}

// --- helpers ---

func activeProduct(merchantID string, price int64) *product.Product {
	return &product.Product{
		ID:         uuid.New(),
		MerchantID: merchantID,
		Name:       "Widget",
		Price:      price,
		Currency:   "USD",
		Status:     product.StatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func authorizedTx(merchantID, orderID, cardToken string, amount int64, currency string, productID uuid.UUID, key string) *transaction.Transaction {
	tx := transaction.NewAuthorized(merchantID, orderID, amount, currency, cardToken)
	tx.MarkProductPurchase(key, productID)
	return tx
}

// --- tests ---

func TestPurchase_HappyPath_Approved(t *testing.T) {
	svc, products, auth, cap, _ := newServiceWithFakes(t)
	p := activeProduct("m1", 1999)
	products.getResp = p

	auth.respTx = authorizedTx("m1", "ord1", "tok1", p.Price, p.Currency, p.ID, "K1")
	cap.resp = transaction.CaptureResponse{TransactionID: auth.respTx.ID, Status: transaction.StatusCapturePending}

	resp, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "ord1",
		ProductID:      p.ID,
		CardToken:      "tok1",
		IdempotencyKey: "K1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != transaction.StatusCapturePending {
		t.Errorf("status = %q, want %q", resp.Status, transaction.StatusCapturePending)
	}
	if !products.markCalled {
		t.Error("MarkPurchasedInTx not called on approved path")
	}
	if !cap.called {
		t.Error("Capture not called on approved path")
	}
	if cap.gotReq.IdempotencyKey != auth.respTx.ID.String()+"-cap" {
		t.Errorf("capture idempotency key = %q, want %s-cap", cap.gotReq.IdempotencyKey, auth.respTx.ID)
	}
	if auth.gotReq.Amount != p.Price || auth.gotReq.Currency != p.Currency {
		t.Errorf("acquirer got amount=%d currency=%s, want %d %s",
			auth.gotReq.Amount, auth.gotReq.Currency, p.Price, p.Currency)
	}
}

func TestPurchase_Declined_NoMarkOrCapture(t *testing.T) {
	svc, products, auth, cap, _ := newServiceWithFakes(t)
	p := activeProduct("m1", 500)
	products.getResp = p

	declined := transaction.NewDeclined("m1", "ord1", p.Price, p.Currency, "tok1", "insufficient_funds")
	declined.MarkProductPurchase("K1", p.ID)
	auth.respTx = declined

	resp, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "ord1",
		ProductID:      p.ID,
		CardToken:      "tok1",
		IdempotencyKey: "K1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != transaction.StatusDeclined {
		t.Errorf("status = %q, want declined", resp.Status)
	}
	if resp.DeclineReason != "insufficient_funds" {
		t.Errorf("decline_reason = %q, want insufficient_funds", resp.DeclineReason)
	}
	if products.markCalled {
		t.Error("MarkPurchasedInTx should not be called on decline")
	}
	if cap.called {
		t.Error("Capture should not be called on decline")
	}
}

func TestPurchase_ArchivedProduct_NoAcquirerCall(t *testing.T) {
	svc, products, auth, _, _ := newServiceWithFakes(t)
	p := activeProduct("m1", 1000)
	p.Status = product.StatusArchived
	products.getResp = p

	_, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "ord1",
		ProductID:      p.ID,
		CardToken:      "tok",
		IdempotencyKey: "K",
	})
	if !errors.Is(err, ErrProductArchived) {
		t.Fatalf("err = %v, want ErrProductArchived", err)
	}
	if auth.called {
		t.Error("acquirer should not be called for archived product")
	}
}

func TestPurchase_ProductNotFound_NoAcquirerCall(t *testing.T) {
	svc, products, auth, _, _ := newServiceWithFakes(t)
	products.getErr = product.ErrNotFound

	_, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		ProductID:      uuid.New(),
		IdempotencyKey: "K",
	})
	if !errors.Is(err, product.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if auth.called {
		t.Error("acquirer should not be called when product missing")
	}
}

func TestPurchase_IdempotencyCacheHit_SameRequest_NoAcquirer(t *testing.T) {
	svc, products, auth, cap, lookup := newServiceWithFakes(t)
	p := activeProduct("m1", 1999)
	cached := authorizedTx("m1", "ord1", "tok1", p.Price, p.Currency, p.ID, "K1")
	lookup.err = nil
	lookup.resp = cached

	resp, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "ord1",
		ProductID:      p.ID,
		CardToken:      "tok1",
		IdempotencyKey: "K1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TransactionID != cached.ID {
		t.Errorf("transaction_id = %s, want cached %s", resp.TransactionID, cached.ID)
	}
	if auth.called {
		t.Error("acquirer should not be called on cache hit")
	}
	if cap.called {
		t.Error("capture should not be called on cache hit")
	}
	if products.getCalled {
		t.Error("product.Get should not be called when cache hits")
	}
}

func TestPurchase_IdempotencyCacheHit_DifferentRequest_Returns409(t *testing.T) {
	svc, _, _, _, lookup := newServiceWithFakes(t)
	cached := authorizedTx("m1", "ord1", "tok1", 100, "USD", uuid.New(), "K1")
	lookup.err = nil
	lookup.resp = cached

	_, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "DIFFERENT",
		ProductID:      *cached.ProductID, // matches cached, mismatch on order_id
		CardToken:      "tok1",
		IdempotencyKey: "K1",
	})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrIdempotencyConflict", err)
	}
}

func TestPurchase_InsertRace_ResolvesByReFetch(t *testing.T) {
	svc, products, auth, cap, lookup := newServiceWithFakes(t)
	p := activeProduct("m1", 1999)
	products.getResp = p

	// Two lookup calls: first (pre-check) → not found, second (race resolve) → cached
	cached := authorizedTx("m1", "ord1", "tok1", p.Price, p.Currency, p.ID, "K1")
	lookup.respSeq = []lookupResult{
		{tx: nil, err: transaction.ErrNotFound},
		{tx: cached, err: nil},
	}

	auth.respErr = transaction.ErrPurchaseIdempotencyConflict

	resp, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "ord1",
		ProductID:      p.ID,
		CardToken:      "tok1",
		IdempotencyKey: "K1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TransactionID != cached.ID {
		t.Errorf("transaction_id = %s, want cached %s", resp.TransactionID, cached.ID)
	}
	if cap.called {
		t.Error("capture should not run on race resolution")
	}
}

func TestPurchase_AcquirerTransportError_Bubbles(t *testing.T) {
	svc, products, auth, _, _ := newServiceWithFakes(t)
	products.getResp = activeProduct("m1", 1000)

	wantErr := errors.New("acquirer down")
	auth.respErr = wantErr

	_, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "o",
		ProductID:      products.getResp.ID,
		CardToken:      "tok",
		IdempotencyKey: "K",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestPurchase_CaptureFailure_ReturnsPartial(t *testing.T) {
	svc, products, auth, cap, _ := newServiceWithFakes(t)
	p := activeProduct("m1", 1999)
	products.getResp = p
	auth.respTx = authorizedTx("m1", "ord1", "tok1", p.Price, p.Currency, p.ID, "K1")
	cap.err = errors.New("capture exploded")

	resp, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "ord1",
		ProductID:      p.ID,
		CardToken:      "tok1",
		IdempotencyKey: "K1",
	})
	if !errors.Is(err, ErrCapturePartiallyApplied) {
		t.Fatalf("err = %v, want ErrCapturePartiallyApplied", err)
	}
	if resp.TransactionID != auth.respTx.ID {
		t.Errorf("partial response missing transaction_id")
	}
	if resp.Status != transaction.StatusAuthorized {
		t.Errorf("partial response status = %q, want authorized", resp.Status)
	}
}

func TestPurchase_MarkPurchasedError_RollsBack(t *testing.T) {
	svc, products, auth, cap, _ := newServiceWithFakes(t)
	p := activeProduct("m1", 1999)
	products.getResp = p
	auth.respTx = authorizedTx("m1", "ord1", "tok1", p.Price, p.Currency, p.ID, "K1")
	products.markErr = errors.New("mark failed")

	_, err := svc.Purchase(context.Background(), Request{
		MerchantID:     "m1",
		OrderID:        "ord1",
		ProductID:      p.ID,
		CardToken:      "tok1",
		IdempotencyKey: "K1",
	})
	if err == nil {
		t.Fatal("expected error from mark failure")
	}
	if cap.called {
		t.Error("capture should not be invoked when tx rolled back")
	}
}
