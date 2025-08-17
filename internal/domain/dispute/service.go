package dispute

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type DisputeService struct {
	disputeRepo DisputeRepo
}

func NewDisputeService(repo DisputeRepo) *DisputeService {
	return &DisputeService{
		disputeRepo: repo,
	}
}

func (s *DisputeService) GetDisputes(ctx context.Context) ([]Dispute, error) {
	disputes, err := s.disputeRepo.GetDisputes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get disputes: %w", err)
	}
	return disputes, nil
}

func (s *DisputeService) GetDisputeByID(ctx context.Context, disputeID string) (*Dispute, error) {
	dispute, err := s.disputeRepo.GetDisputeByID(ctx, disputeID)
	if err != nil {
		return nil, fmt.Errorf("get dispute by id: %w", err)
	}
	return dispute, err
}

func (s *DisputeService) GetEvents(ctx context.Context, disputeID string) ([]DisputeEvent, error) {
	query := NewDisputeEventQueryBuilder().
		WithDisputeIDs(disputeID).
		Build()

	events, err := s.disputeRepo.GetDisputeEvents(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get events for dispute %s: %w", disputeID, err)
	}
	return events, nil
}

func (s *DisputeService) ProcessChargeback(ctx context.Context, webhook ChargebackWebhook) error {
	return s.disputeRepo.InTransaction(ctx, func(tx TxDisputeRepo) error {
		dispute, err := tx.GetDisputeByOrderID(ctx, webhook.OrderID)
		if err != nil {
			return fmt.Errorf("get dispute by order_id: %w", err)
		}

		if dispute == nil {
			if webhook.Status != ChargebackOpened {
				return fmt.Errorf("dispute not found for order_id: %s", webhook.OrderID)
			}

			newDispute := NewDispute{
				Status: DisputeOpen,
				DisputeInfo: DisputeInfo{
					OrderID:       webhook.OrderID,
					Reason:        webhook.Reason,
					Money:         webhook.Money,
					OpenedAt:      webhook.OccurredAt,
					EvidenceDueAt: webhook.EvidenceDueAt,
					SubmittedAt:   nil, //todo comment
					ClosedAt:      nil, //todo comment
				},
			}
			created, err := tx.CreateDispute(ctx, newDispute)
			if err != nil {
				return fmt.Errorf("create dispute: %w", err)
			}
			if err := s.saveWebhookEvent(ctx, tx, *created, webhook); err != nil {
				return fmt.Errorf("create dispute event: %w", err)
			}

			return nil
		}

		updated, err := ApplyChargebackWebhook(*dispute, webhook)
		if err != nil {
			return err
		}

		if err := tx.UpdateDispute(ctx, updated); err != nil {
			return fmt.Errorf("update dispute: %w", err)
		}

		if err := s.saveWebhookEvent(ctx, tx, *dispute, webhook); err != nil {
			return fmt.Errorf("create dispute event: %w", err)
		}

		return nil
	})
}

func (s *DisputeService) UpsertEvidence(ctx context.Context, disputeID string, upsert EvidenceUpsert) (*Evidence, error) {
	var result *Evidence

	err := s.disputeRepo.InTransaction(ctx, func(tx TxDisputeRepo) error {
		// 1. Validate that dispute exists and is editable
		dispute, err := tx.GetDisputeByID(ctx, disputeID)
		if err != nil {
			return fmt.Errorf("get dispute by id: %w", err)
		}
		if dispute == nil {
			return fmt.Errorf("dispute not found")
		}

		if !IsDisputeEditable(dispute.Status) {
			return fmt.Errorf("dispute cannot be edited in status: %s", dispute.Status)
		}

		// 2. Upsert evidence
		evidence, err := tx.UpsertEvidence(ctx, disputeID, upsert)
		if err != nil {
			return fmt.Errorf("upsert evidence: %w", err)
		}
		result = evidence

		// 3. Update dispute status from open to under_review if needed
		if dispute.Status == DisputeOpen {
			dispute.Status = DisputeUnderReview
			if err := tx.UpdateDispute(ctx, *dispute); err != nil {
				return fmt.Errorf("update dispute status: %w", err)
			}
		}

		// 4. Create evidence_added event
		if err := s.createEvidenceAddedEvent(ctx, tx, disputeID, *evidence); err != nil {
			return fmt.Errorf("create evidence event: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *DisputeService) createEvidenceAddedEvent(ctx context.Context, tx TxDisputeRepo, disputeID string, evidence Evidence) error {
	payload, _ := json.Marshal(evidence)

	disputeEvent := NewDisputeEvent{
		DisputeID:       disputeID,
		Kind:            DisputeEventEvidenceAdded,
		ProviderEventID: "", // No provider event for evidence added by merchant
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	if err := tx.CreateDisputeEvent(ctx, disputeEvent); err != nil {
		return fmt.Errorf("create dispute event: %w", err)
	}
	return nil
}

func (s *DisputeService) saveWebhookEvent(ctx context.Context, tx TxDisputeRepo, dispute Dispute, webhook ChargebackWebhook) error {
	payload, _ := json.Marshal(webhook)

	disputeEvent := NewDisputeEvent{
		DisputeID:       dispute.ID,
		Kind:            deriveKindFromChargebackStatus(webhook.Status),
		ProviderEventID: webhook.ProviderEventID,
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	if err := tx.CreateDisputeEvent(ctx, disputeEvent); err != nil {
		return fmt.Errorf("create dispute event: %w", err)
	}
	return nil
}
