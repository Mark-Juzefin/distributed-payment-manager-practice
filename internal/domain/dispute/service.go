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

func (s *DisputeService) GetDisputeByID(ctx context.Context, disputeID string) (*Dispute, error) {
	dispute, err := s.disputeRepo.GetDisputeByID(ctx, disputeID)
	if err != nil {
		return nil, fmt.Errorf("get dispute by id: %w", err)
	}
	return dispute, err
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
