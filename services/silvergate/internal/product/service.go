package product

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

type Service struct {
	repo Repo
	log  *slog.Logger
}

func NewService(repo Repo, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

type CreateInput struct {
	Slug        *string
	Name        string
	Description string
	Price       int64
	Currency    string
}

func (s *Service) Create(ctx context.Context, merchantID string, in CreateInput) (*Product, error) {
	if in.Slug != nil {
		if err := ValidateSlug(*in.Slug); err != nil {
			return nil, err
		}
	}
	p := New(merchantID, in.Name, in.Description, in.Price, in.Currency, in.Slug)
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("save product: %w", err)
	}
	s.log.Info("product created",
		"product_id", p.ID,
		"merchant_id", merchantID,
	)
	return p, nil
}

func (s *Service) Get(ctx context.Context, merchantID string, id uuid.UUID) (*Product, error) {
	return s.repo.GetByID(ctx, merchantID, id)
}

func (s *Service) List(ctx context.Context, merchantID string, filter ListFilter) ([]*Product, *Cursor, error) {
	if filter.Limit <= 0 {
		filter.Limit = defaultListLimit
	}
	if filter.Limit > maxListLimit {
		return nil, nil, ErrLimitTooLarge
	}
	return s.repo.List(ctx, merchantID, filter)
}

func (s *Service) Update(ctx context.Context, merchantID string, id uuid.UUID, req UpdateRequest) (*Product, error) {
	p, err := s.repo.GetByID(ctx, merchantID, id)
	if err != nil {
		return nil, err
	}

	upd, err := NewUpdate(req, p)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, merchantID, id, upd); err != nil {
		return nil, err
	}

	s.log.Info("product updated",
		"product_id", id,
		"merchant_id", merchantID,
	)
	return s.repo.GetByID(ctx, merchantID, id)
}

func (s *Service) Archive(ctx context.Context, merchantID string, id uuid.UUID) error {
	return s.repo.SetStatus(ctx, merchantID, id, StatusArchived)
}

func (s *Service) Unarchive(ctx context.Context, merchantID string, id uuid.UUID) error {
	return s.repo.SetStatus(ctx, merchantID, id, StatusActive)
}

// MarkPurchased is the entry point for /purchase (Subtask 2). Idempotent.
func (s *Service) MarkPurchased(ctx context.Context, merchantID string, id uuid.UUID) error {
	return s.repo.MarkPurchased(ctx, merchantID, id)
}
