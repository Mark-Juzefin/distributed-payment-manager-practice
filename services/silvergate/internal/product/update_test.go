package product

import (
	"errors"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func newActiveProduct() *Product {
	now := time.Now().UTC()
	return &Product{
		MerchantID:  "m1",
		Slug:        ptr("plan-pro"),
		Name:        "Pro",
		Description: "desc",
		Price:       1000,
		Currency:    "EUR",
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func newPurchasedProduct() *Product {
	p := newActiveProduct()
	t := time.Now().UTC()
	p.FirstPurchasedAt = &t
	return p
}

func newArchivedProduct() *Product {
	p := newActiveProduct()
	p.Status = StatusArchived
	return p
}

func TestNewUpdate(t *testing.T) {
	tests := []struct {
		name      string
		req       UpdateRequest
		product   *Product
		wantErr   error
		wantInfo  bool
		wantLockd bool
	}{
		{
			name:    "empty request rejected",
			req:     UpdateRequest{},
			product: newActiveProduct(),
			wantErr: ErrEmptyUpdate,
		},
		{
			name:     "name-only sets Info group",
			req:      UpdateRequest{Name: ptr("New")},
			product:  newActiveProduct(),
			wantInfo: true,
		},
		{
			name:     "description-only sets Info group",
			req:      UpdateRequest{Description: ptr("d2")},
			product:  newActiveProduct(),
			wantInfo: true,
		},
		{
			name:      "price-only sets Locked group",
			req:       UpdateRequest{Price: ptr(int64(2000))},
			product:   newActiveProduct(),
			wantLockd: true,
		},
		{
			name:      "slug + price sets Locked",
			req:       UpdateRequest{Slug: ptr("new-slug"), Price: ptr(int64(2000))},
			product:   newActiveProduct(),
			wantLockd: true,
		},
		{
			name:      "mixed: name + price → both groups",
			req:       UpdateRequest{Name: ptr("X"), Price: ptr(int64(99))},
			product:   newActiveProduct(),
			wantInfo:  true,
			wantLockd: true,
		},
		{
			name:    "archived product → ErrArchived (even info-only)",
			req:     UpdateRequest{Name: ptr("X")},
			product: newArchivedProduct(),
			wantErr: ErrArchived,
		},
		{
			name:    "purchased product + locked field → ErrFieldsLocked",
			req:     UpdateRequest{Price: ptr(int64(99))},
			product: newPurchasedProduct(),
			wantErr: ErrFieldsLocked,
		},
		{
			name:     "purchased product + info-only allowed",
			req:      UpdateRequest{Name: ptr("renamed")},
			product:  newPurchasedProduct(),
			wantInfo: true,
		},
		{
			name:    "slug downgrade to empty string rejected",
			req:     UpdateRequest{Slug: ptr("")},
			product: newActiveProduct(),
			wantErr: ErrSlugRemoval,
		},
		{
			name:    "invalid slug format rejected",
			req:     UpdateRequest{Slug: ptr("BadSlug!")},
			product: newActiveProduct(),
			wantErr: ErrInvalidSlug,
		},
		{
			name:      "valid slug accepted",
			req:       UpdateRequest{Slug: ptr("new-slug-2")},
			product:   newActiveProduct(),
			wantLockd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upd, err := NewUpdate(tt.req, tt.product)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want err %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tt.wantInfo && upd.Info == nil {
				t.Errorf("expected Info group, got nil")
			}
			if !tt.wantInfo && upd.Info != nil {
				t.Errorf("expected no Info group, got %+v", upd.Info)
			}
			if tt.wantLockd && upd.Locked == nil {
				t.Errorf("expected Locked group, got nil")
			}
			if !tt.wantLockd && upd.Locked != nil {
				t.Errorf("expected no Locked group, got %+v", upd.Locked)
			}
		})
	}
}

func TestLockedAfterPurchase_FieldNames(t *testing.T) {
	got := LockedAfterPurchase{}.FieldNames()
	want := []string{"slug", "price", "currency"}
	if len(got) != len(want) {
		t.Fatalf("want %d fields, got %d (%v)", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("idx %d: want %q, got %q", i, w, got[i])
		}
	}
}

func TestProduct_LockedFields(t *testing.T) {
	p := newActiveProduct()
	if got := p.LockedFields(); got != nil {
		t.Errorf("active product: want nil, got %v", got)
	}

	pp := newPurchasedProduct()
	got := pp.LockedFields()
	if len(got) != 3 {
		t.Errorf("purchased product: want 3 fields, got %v", got)
	}
}

func TestValidateSlug(t *testing.T) {
	cases := []struct {
		slug string
		ok   bool
	}{
		{"a", true},
		{"plan-pro", true},
		{"plan-pro-1", true},
		{"a1-b2", true},
		{"", false},
		{"-leading", false},
		{"trailing-", false},
		{"double--dash", false},
		{"UpperCase", false},
		{"with space", false},
		{"with_underscore", false},
	}
	for _, c := range cases {
		err := ValidateSlug(c.slug)
		if c.ok && err != nil {
			t.Errorf("slug %q: want ok, got %v", c.slug, err)
		}
		if !c.ok && err == nil {
			t.Errorf("slug %q: want err, got nil", c.slug)
		}
	}
}
