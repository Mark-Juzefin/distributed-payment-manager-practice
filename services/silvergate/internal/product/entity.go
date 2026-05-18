package product

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type Product struct {
	ID               uuid.UUID
	MerchantID       string
	Slug             *string
	Name             string
	Description      string
	Price            int64
	Currency         string
	Status           Status
	FirstPurchasedAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func New(merchantID, name, description string, price int64, currency string, slug *string) *Product {
	now := time.Now().UTC()
	return &Product{
		ID:          uuid.New(),
		MerchantID:  merchantID,
		Slug:        slug,
		Name:        name,
		Description: description,
		Price:       price,
		Currency:    currency,
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (p *Product) LockedFields() []string {
	if p.FirstPurchasedAt == nil {
		return nil
	}
	return LockedAfterPurchase{}.FieldNames()
}

func (p *Product) IsArchived() bool {
	return p.Status == StatusArchived
}

var slugRegex = regexp.MustCompile(`^[a-z0-9](-?[a-z0-9])*$`)

func ValidateSlug(s string) error {
	if len(s) < 1 || len(s) > 64 {
		return ErrInvalidSlug
	}
	if !slugRegex.MatchString(s) {
		return ErrInvalidSlug
	}
	return nil
}
