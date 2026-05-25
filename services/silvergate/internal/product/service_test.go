package product

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

// fakeRepo is a hand-rolled Repo stub. Each method either returns a queued
// value or records the arguments. Tests assert via the recorded fields.
type fakeRepo struct {
	createErr error
	createGot *Product

	getProduct *Product
	getErr     error
	getCalls   int

	listItems  []*Product
	listCursor *Cursor
	listErr    error
	listGot    ListFilter

	updateErr error
	updateGot Update

	setStatusErr    error
	setStatusGot    Status
	setStatusCalled bool

	markPurchasedErr    error
	markPurchasedCalled bool
}

func (r *fakeRepo) Create(_ context.Context, p *Product) error {
	r.createGot = p
	return r.createErr
}
func (r *fakeRepo) GetByID(_ context.Context, _ string, _ uuid.UUID) (*Product, error) {
	r.getCalls++
	return r.getProduct, r.getErr
}
func (r *fakeRepo) List(_ context.Context, _ string, f ListFilter) ([]*Product, *Cursor, error) {
	r.listGot = f
	return r.listItems, r.listCursor, r.listErr
}
func (r *fakeRepo) Update(_ context.Context, _ string, _ uuid.UUID, upd Update) error {
	r.updateGot = upd
	return r.updateErr
}
func (r *fakeRepo) SetStatus(_ context.Context, _ string, _ uuid.UUID, s Status) error {
	r.setStatusCalled = true
	r.setStatusGot = s
	return r.setStatusErr
}
func (r *fakeRepo) MarkPurchased(_ context.Context, _ string, _ uuid.UUID) error {
	r.markPurchasedCalled = true
	return r.markPurchasedErr
}

func newService(repo Repo) *Service {
	return NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestService_Create(t *testing.T) {
	t.Run("invalid slug rejected before repo.Create", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := newService(repo)
		_, err := svc.Create(context.Background(), "m1", CreateInput{
			Slug:     ptr("Bad Slug"),
			Name:     "X",
			Price:    100,
			Currency: "EUR",
		})
		if !errors.Is(err, ErrInvalidSlug) {
			t.Fatalf("want ErrInvalidSlug, got %v", err)
		}
		if repo.createGot != nil {
			t.Error("repo.Create should not be called on slug validation error")
		}
	})

	t.Run("valid input persists product with merchant id", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := newService(repo)
		p, err := svc.Create(context.Background(), "m1", CreateInput{
			Slug: ptr("plan-a"), Name: "A", Price: 100, Currency: "EUR",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.MerchantID != "m1" {
			t.Errorf("merchant id: want m1, got %q", p.MerchantID)
		}
		if repo.createGot == nil || repo.createGot.ID != p.ID {
			t.Error("repo.Create not called with constructed product")
		}
	})

	t.Run("repo error wrapped", func(t *testing.T) {
		repo := &fakeRepo{createErr: ErrSlugConflict}
		svc := newService(repo)
		_, err := svc.Create(context.Background(), "m1", CreateInput{
			Name: "A", Price: 100, Currency: "EUR",
		})
		if !errors.Is(err, ErrSlugConflict) {
			t.Fatalf("want ErrSlugConflict, got %v", err)
		}
	})
}

func TestService_List(t *testing.T) {
	t.Run("default limit applied when 0", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := newService(repo)
		_, _, err := svc.List(context.Background(), "m1", ListFilter{})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if repo.listGot.Limit != defaultListLimit {
			t.Errorf("want default limit %d, got %d", defaultListLimit, repo.listGot.Limit)
		}
	})

	t.Run("limit over max rejected", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := newService(repo)
		_, _, err := svc.List(context.Background(), "m1", ListFilter{Limit: maxListLimit + 1})
		if !errors.Is(err, ErrLimitTooLarge) {
			t.Fatalf("want ErrLimitTooLarge, got %v", err)
		}
	})

	t.Run("filter passed through", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := newService(repo)
		st := StatusArchived
		_, _, _ = svc.List(context.Background(), "m1", ListFilter{
			StatusFilter: &st, Limit: 10,
		})
		if repo.listGot.StatusFilter == nil || *repo.listGot.StatusFilter != StatusArchived {
			t.Errorf("status filter not forwarded: %v", repo.listGot.StatusFilter)
		}
		if repo.listGot.Limit != 10 {
			t.Errorf("want limit 10, got %d", repo.listGot.Limit)
		}
	})
}

func TestService_Update(t *testing.T) {
	t.Run("fetches snapshot before NewUpdate", func(t *testing.T) {
		repo := &fakeRepo{getProduct: newActiveProduct()}
		svc := newService(repo)
		_, err := svc.Update(context.Background(), "m1", uuid.New(), UpdateRequest{
			Name: ptr("renamed"),
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if repo.getCalls < 2 {
			t.Errorf("want at least 2 GetByID calls (pre+post), got %d", repo.getCalls)
		}
		if repo.updateGot.Info == nil || repo.updateGot.Info.Name == nil {
			t.Error("Update Info.Name not forwarded")
		}
	})

	t.Run("snapshot fetch error returned", func(t *testing.T) {
		repo := &fakeRepo{getErr: ErrNotFound}
		svc := newService(repo)
		_, err := svc.Update(context.Background(), "m1", uuid.New(), UpdateRequest{
			Name: ptr("x"),
		})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("NewUpdate error returned without calling repo.Update", func(t *testing.T) {
		repo := &fakeRepo{getProduct: newPurchasedProduct()}
		svc := newService(repo)
		_, err := svc.Update(context.Background(), "m1", uuid.New(), UpdateRequest{
			Price: ptr(int64(999)),
		})
		if !errors.Is(err, ErrFieldsLocked) {
			t.Fatalf("want ErrFieldsLocked, got %v", err)
		}
		if repo.updateGot.Info != nil || repo.updateGot.Locked != nil {
			t.Error("repo.Update should not have been invoked")
		}
	})
}

func TestService_ArchiveUnarchive(t *testing.T) {
	t.Run("Archive sets status archived", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := newService(repo)
		if err := svc.Archive(context.Background(), "m1", uuid.New()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !repo.setStatusCalled || repo.setStatusGot != StatusArchived {
			t.Errorf("want SetStatus(archived), got called=%v status=%v",
				repo.setStatusCalled, repo.setStatusGot)
		}
	})

	t.Run("Unarchive sets status active", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := newService(repo)
		if err := svc.Unarchive(context.Background(), "m1", uuid.New()); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if repo.setStatusGot != StatusActive {
			t.Errorf("want active, got %v", repo.setStatusGot)
		}
	})
}

func TestService_MarkPurchased(t *testing.T) {
	repo := &fakeRepo{}
	svc := newService(repo)
	if err := svc.MarkPurchased(context.Background(), "m1", uuid.New()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !repo.markPurchasedCalled {
		t.Error("MarkPurchased not forwarded to repo")
	}
}
