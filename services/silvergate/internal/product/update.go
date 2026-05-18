package product

// UpdateRequest is the raw partial-update payload. Nil pointer = unchanged.
type UpdateRequest struct {
	Name        *string
	Description *string
	Slug        *string
	Price       *int64
	Currency    *string
}

type InfoUpdate struct {
	Name        *string
	Description *string
}

// LockedAfterPurchase groups fields that become immutable after first purchase.
// Struct fields here + FieldNames() are the single source of truth.
type LockedAfterPurchase struct {
	Slug     *string
	Price    *int64
	Currency *string
}

func (LockedAfterPurchase) FieldNames() []string {
	return []string{"slug", "price", "currency"}
}

// Update is a validated payload accepted by Repo. nil group = no changes for that group.
type Update struct {
	Info   *InfoUpdate
	Locked *LockedAfterPurchase
}

// NewUpdate is the only legitimate constructor for Update.
func NewUpdate(req UpdateRequest, p *Product) (Update, error) {
	var upd Update

	if req.Name != nil || req.Description != nil {
		upd.Info = &InfoUpdate{
			Name:        req.Name,
			Description: req.Description,
		}
	}
	if req.Slug != nil || req.Price != nil || req.Currency != nil {
		upd.Locked = &LockedAfterPurchase{
			Slug:     req.Slug,
			Price:    req.Price,
			Currency: req.Currency,
		}
	}

	if upd.Info == nil && upd.Locked == nil {
		return Update{}, ErrEmptyUpdate
	}
	if p.IsArchived() {
		return Update{}, ErrArchived
	}
	if upd.Locked != nil && p.FirstPurchasedAt != nil {
		return Update{}, ErrFieldsLocked
	}
	if upd.Locked != nil && upd.Locked.Slug != nil {
		if *upd.Locked.Slug == "" {
			return Update{}, ErrSlugRemoval
		}
		if err := ValidateSlug(*upd.Locked.Slug); err != nil {
			return Update{}, err
		}
	}

	return upd, nil
}
