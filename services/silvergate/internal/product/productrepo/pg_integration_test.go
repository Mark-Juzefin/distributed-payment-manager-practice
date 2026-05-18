//go:build integration

package productrepo_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/product/productrepo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// merchantID returns a unique merchant ID per test to isolate row sets across t.Parallel runs.
func merchantID(t *testing.T) string {
	t.Helper()
	return "m_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func ptr[T any](v T) *T { return &v }

// seed inserts a Product directly via the repo; callers customise via opts.
func seed(t *testing.T, ctx context.Context, repo *productrepo.PgProductRepo, merchant string, opts ...func(*product.Product)) *product.Product {
	t.Helper()
	p := product.New(merchant, "Widget", "Basic widget", 1500, "EUR", nil)
	for _, o := range opts {
		o(p)
	}
	require.NoError(t, repo.Create(ctx, p))
	return p
}

func TestCreate_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := productrepo.NewPgProductRepo(pg.Pool)
	merchant := merchantID(t)

	p := product.New(merchant, "Premium plan", "Annual sub", 9900, "USD", ptr("premium"))
	require.NoError(t, repo.Create(ctx, p))

	got, err := repo.GetByID(ctx, merchant, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.Name, got.Name)
	assert.Equal(t, p.Price, got.Price)
	assert.Equal(t, p.Currency, got.Currency)
	require.NotNil(t, got.Slug)
	assert.Equal(t, "premium", *got.Slug)
	assert.Equal(t, product.StatusActive, got.Status)
	assert.Nil(t, got.FirstPurchasedAt)
}

func TestCreate_ConstraintsAndIsolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T, merchant, otherMerchant string) *product.Product
		mutate    func(p *product.Product, merchant, otherMerchant string)
		wantErr   error // domain error, or nil if expecting raw DB error
		wantDBErr bool  // expect CHECK constraint violation (pg code starts 23)
	}{
		{
			name:    "duplicate slug for same merchant rejected",
			mutate:  func(p *product.Product, m, _ string) { p.Slug = ptr("dup-slug") },
			wantErr: product.ErrSlugConflict,
		},
		{
			name: "same slug across merchants allowed",
			setup: func(t *testing.T, m, om string) *product.Product {
				p := product.New(m, "Original", "", 1000, "EUR", ptr("shared-slug"))
				return p
			},
			mutate: func(p *product.Product, _, om string) {
				p.ID = uuid.New()
				p.MerchantID = om
				p.Slug = ptr("shared-slug")
			},
		},
		{
			name:   "two products with nil slug for same merchant allowed",
			setup:  func(t *testing.T, m, _ string) *product.Product { return product.New(m, "A", "", 1000, "EUR", nil) },
			mutate: func(p *product.Product, _, _ string) { p.ID = uuid.New(); p.Slug = nil },
		},
		{
			name:      "price <= 0 violates CHECK",
			mutate:    func(p *product.Product, _, _ string) { p.Price = 0 },
			wantDBErr: true,
		},
		{
			name:      "currency length != 3 violates CHECK",
			mutate:    func(p *product.Product, _, _ string) { p.Currency = "EURO" },
			wantDBErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			repo := productrepo.NewPgProductRepo(pg.Pool)
			merchant := merchantID(t)
			otherMerchant := merchantID(t)

			var first *product.Product
			if tt.setup != nil {
				first = tt.setup(t, merchant, otherMerchant)
			} else {
				first = product.New(merchant, "Widget", "", 1500, "EUR", ptr("dup-slug"))
			}
			require.NoError(t, repo.Create(ctx, first))

			second := *first // shallow copy is fine; pointer fields will be re-set
			second.ID = uuid.New()
			tt.mutate(&second, merchant, otherMerchant)

			err := repo.Create(ctx, &second)
			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantDBErr:
				require.Error(t, err)
				var pgErr *pgconn.PgError
				require.ErrorAs(t, err, &pgErr)
				assert.True(t, strings.HasPrefix(pgErr.Code, "23"), "expected integrity violation, got code %s", pgErr.Code)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestGetByID_CrossMerchantIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := productrepo.NewPgProductRepo(pg.Pool)
	merchant := merchantID(t)
	intruder := merchantID(t)

	p := seed(t, ctx, repo, merchant)

	_, err := repo.GetByID(ctx, intruder, p.ID)
	assert.ErrorIs(t, err, product.ErrNotFound)

	_, err = repo.GetByID(ctx, merchant, uuid.New())
	assert.ErrorIs(t, err, product.ErrNotFound)
}

func TestList_PaginationAndFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := productrepo.NewPgProductRepo(pg.Pool)
	merchant := merchantID(t)

	// Five active products with strictly increasing created_at to make ordering deterministic.
	base := time.Now().UTC().Add(-time.Hour)
	created := make([]*product.Product, 5)
	for i := range created {
		p := product.New(merchant, "P", "", int64((i+1)*100), "EUR", nil)
		p.CreatedAt = base.Add(time.Duration(i) * time.Second)
		p.UpdatedAt = p.CreatedAt
		require.NoError(t, repo.Create(ctx, p))
		created[i] = p
	}
	// Plus one archived row to verify status filter.
	archived := product.New(merchant, "Archived", "", 500, "EUR", nil)
	archived.Status = product.StatusArchived
	archived.CreatedAt = base.Add(10 * time.Second)
	archived.UpdatedAt = archived.CreatedAt
	require.NoError(t, repo.Create(ctx, archived))

	t.Run("first page returns newest active rows with next cursor", func(t *testing.T) {
		active := product.StatusActive
		page, next, err := repo.List(ctx, merchant, product.ListFilter{StatusFilter: &active, Limit: 3})
		require.NoError(t, err)
		require.Len(t, page, 3)
		// Ordered DESC by created_at: index 4, 3, 2 of `created`.
		assert.Equal(t, created[4].ID, page[0].ID)
		assert.Equal(t, created[3].ID, page[1].ID)
		assert.Equal(t, created[2].ID, page[2].ID)
		require.NotNil(t, next)
		assert.Equal(t, created[2].ID, next.ID)
	})

	t.Run("second page resumes from cursor and exhausts results", func(t *testing.T) {
		active := product.StatusActive
		page, next, err := repo.List(ctx, merchant, product.ListFilter{
			StatusFilter: &active,
			Limit:        3,
			Cursor:       &product.Cursor{CreatedAt: created[2].CreatedAt, ID: created[2].ID},
		})
		require.NoError(t, err)
		require.Len(t, page, 2)
		assert.Equal(t, created[1].ID, page[0].ID)
		assert.Equal(t, created[0].ID, page[1].ID)
		assert.Nil(t, next, "no more rows → cursor should be nil")
	})

	t.Run("nil status filter returns active + archived", func(t *testing.T) {
		page, _, err := repo.List(ctx, merchant, product.ListFilter{Limit: 100})
		require.NoError(t, err)
		assert.Len(t, page, 6)
	})

	t.Run("archived filter returns only archived row", func(t *testing.T) {
		filterStatus := product.StatusArchived
		page, _, err := repo.List(ctx, merchant, product.ListFilter{StatusFilter: &filterStatus, Limit: 10})
		require.NoError(t, err)
		require.Len(t, page, 1)
		assert.Equal(t, archived.ID, page[0].ID)
	})
}

func TestUpdate_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		seedOpts []func(*product.Product) // applied before create
		req      product.UpdateRequest
		// markPurchasedBeforeUpdate forces row into "purchased" state to test locked-fields path.
		markPurchasedBeforeUpdate bool
		archiveBeforeUpdate       bool
		callerMerchant            string // empty → use real owner
		wantErr                   error
		check                     func(t *testing.T, before, after *product.Product)
	}{
		{
			name: "update info fields only",
			req:  product.UpdateRequest{Name: ptr("New name"), Description: ptr("New desc")},
			check: func(t *testing.T, _, after *product.Product) {
				assert.Equal(t, "New name", after.Name)
				assert.Equal(t, "New desc", after.Description)
				assert.Equal(t, int64(1500), after.Price)
			},
		},
		{
			name: "update locked fields when not purchased",
			req:  product.UpdateRequest{Price: ptr(int64(2500)), Currency: ptr("USD"), Slug: ptr("new-slug")},
			check: func(t *testing.T, _, after *product.Product) {
				assert.Equal(t, int64(2500), after.Price)
				assert.Equal(t, "USD", after.Currency)
				require.NotNil(t, after.Slug)
				assert.Equal(t, "new-slug", *after.Slug)
			},
		},
		{
			name:                      "update locked fields after purchase returns ErrFieldsLocked",
			req:                       product.UpdateRequest{Price: ptr(int64(9999))},
			markPurchasedBeforeUpdate: true,
			wantErr:                   product.ErrFieldsLocked,
		},
		{
			name:                      "update info-only fields after purchase still succeeds",
			req:                       product.UpdateRequest{Name: ptr("Renamed")},
			markPurchasedBeforeUpdate: true,
			check: func(t *testing.T, _, after *product.Product) {
				assert.Equal(t, "Renamed", after.Name)
				assert.NotNil(t, after.FirstPurchasedAt)
			},
		},
		{
			name:                "update on archived product returns ErrArchived",
			req:                 product.UpdateRequest{Name: ptr("Cannot")},
			archiveBeforeUpdate: true,
			wantErr:             product.ErrArchived,
		},
		{
			name:           "cross-merchant update returns ErrNotFound",
			req:            product.UpdateRequest{Name: ptr("Stolen")},
			callerMerchant: "intruder",
			wantErr:        product.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			repo := productrepo.NewPgProductRepo(pg.Pool)
			merchant := merchantID(t)
			caller := merchant
			if tt.callerMerchant != "" {
				caller = merchantID(t) // unique stranger
			}

			before := seed(t, ctx, repo, merchant, tt.seedOpts...)

			if tt.archiveBeforeUpdate {
				require.NoError(t, repo.SetStatus(ctx, merchant, before.ID, product.StatusArchived))
			}
			if tt.markPurchasedBeforeUpdate {
				require.NoError(t, repo.MarkPurchased(ctx, merchant, before.ID))
			}

			// Service-level path: re-fetch + NewUpdate + repo.Update, mirroring Service.Update so
			// validation errors surface from NewUpdate where applicable.
			latest, err := repo.GetByID(ctx, merchant, before.ID)
			if errors.Is(err, product.ErrNotFound) && tt.callerMerchant == "" {
				t.Fatalf("seeded row vanished: %v", err)
			}
			if tt.callerMerchant != "" {
				// Caller is an intruder — emulate cross-merchant path directly via repo.Update.
				// We still need *something* to construct an Update from, so reuse `before`.
				upd, buildErr := product.NewUpdate(tt.req, before)
				require.NoError(t, buildErr)
				err := repo.Update(ctx, caller, before.ID, upd)
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			upd, buildErr := product.NewUpdate(tt.req, latest)
			if tt.wantErr != nil && (errors.Is(tt.wantErr, product.ErrArchived) || errors.Is(tt.wantErr, product.ErrFieldsLocked)) {
				require.ErrorIs(t, buildErr, tt.wantErr)
				return
			}
			require.NoError(t, buildErr)

			err = repo.Update(ctx, caller, before.ID, upd)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			after, err := repo.GetByID(ctx, merchant, before.ID)
			require.NoError(t, err)
			assert.True(t, after.UpdatedAt.After(before.UpdatedAt) || after.UpdatedAt.Equal(before.UpdatedAt))
			if tt.check != nil {
				tt.check(t, before, after)
			}
		})
	}
}

func TestUpdate_SlugConflictMapsToErrSlugConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := productrepo.NewPgProductRepo(pg.Pool)
	merchant := merchantID(t)

	first := product.New(merchant, "A", "", 1000, "EUR", ptr("taken"))
	require.NoError(t, repo.Create(ctx, first))
	second := product.New(merchant, "B", "", 1000, "EUR", nil)
	require.NoError(t, repo.Create(ctx, second))

	upd, err := product.NewUpdate(product.UpdateRequest{Slug: ptr("taken")}, second)
	require.NoError(t, err)
	err = repo.Update(ctx, merchant, second.ID, upd)
	assert.ErrorIs(t, err, product.ErrSlugConflict)
}

func TestSetStatus_IdempotentAndNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := productrepo.NewPgProductRepo(pg.Pool)
	merchant := merchantID(t)
	p := seed(t, ctx, repo, merchant)

	// active → archived
	require.NoError(t, repo.SetStatus(ctx, merchant, p.ID, product.StatusArchived))
	got, _ := repo.GetByID(ctx, merchant, p.ID)
	assert.Equal(t, product.StatusArchived, got.Status)

	// archived → archived (no-op, must not error per decision #8)
	require.NoError(t, repo.SetStatus(ctx, merchant, p.ID, product.StatusArchived))

	// archived → active
	require.NoError(t, repo.SetStatus(ctx, merchant, p.ID, product.StatusActive))

	// unknown id
	err := repo.SetStatus(ctx, merchant, uuid.New(), product.StatusArchived)
	assert.ErrorIs(t, err, product.ErrNotFound)

	// cross-merchant
	err = repo.SetStatus(ctx, merchantID(t), p.ID, product.StatusArchived)
	assert.ErrorIs(t, err, product.ErrNotFound)
}

func TestMarkPurchased_IdempotentAndNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := productrepo.NewPgProductRepo(pg.Pool)
	merchant := merchantID(t)
	p := seed(t, ctx, repo, merchant)

	require.NoError(t, repo.MarkPurchased(ctx, merchant, p.ID))
	first, err := repo.GetByID(ctx, merchant, p.ID)
	require.NoError(t, err)
	require.NotNil(t, first.FirstPurchasedAt)
	stamped := *first.FirstPurchasedAt

	// Second call must not overwrite the timestamp.
	require.NoError(t, repo.MarkPurchased(ctx, merchant, p.ID))
	second, err := repo.GetByID(ctx, merchant, p.ID)
	require.NoError(t, err)
	require.NotNil(t, second.FirstPurchasedAt)
	assert.True(t, second.FirstPurchasedAt.Equal(stamped), "timestamp must be frozen after first set")

	// Unknown id and cross-merchant both surface as ErrNotFound.
	assert.ErrorIs(t, repo.MarkPurchased(ctx, merchant, uuid.New()), product.ErrNotFound)
	assert.ErrorIs(t, repo.MarkPurchased(ctx, merchantID(t), p.ID), product.ErrNotFound)
}
