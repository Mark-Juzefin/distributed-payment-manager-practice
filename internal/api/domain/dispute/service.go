package dispute

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"TestTaskJustPay/internal/api/domain/gateway"
	"TestTaskJustPay/pkg/pointers"
	"TestTaskJustPay/pkg/postgres"

	"github.com/google/uuid"
)

type DisputeService struct {
	transactor    postgres.Transactor
	txDisputeRepo func(tx postgres.Executor) DisputeRepo
	disputeRepo   DisputeRepo // for reads
	disputeEvents DisputeEvents
	provider      gateway.Provider
}

func NewDisputeService(
	transactor postgres.Transactor,
	txDisputeRepo func(tx postgres.Executor) DisputeRepo,
	disputeRepo DisputeRepo,
	provider gateway.Provider,
	disputeEvents DisputeEvents,
) *DisputeService {
	return &DisputeService{
		transactor:    transactor,
		txDisputeRepo: txDisputeRepo,
		disputeRepo:   disputeRepo,
		provider:      provider,
		disputeEvents: disputeEvents,
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

func (s *DisputeService) GetEvents(ctx context.Context, query DisputeEventQuery) (DisputeEventPage, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}

	eventPage, err := s.disputeEvents.GetDisputeEvents(ctx, query)
	if err != nil {
		return DisputeEventPage{}, fmt.Errorf("get events: %w", err)
	}
	return eventPage, nil
}

func (s *DisputeService) GetEvidence(ctx context.Context, disputeID string) (*Evidence, error) {
	evidence, err := s.disputeRepo.GetEvidence(ctx, disputeID)
	if err != nil {
		return nil, fmt.Errorf("get evidence for dispute %s: %w", disputeID, err)
	}
	return evidence, nil
}

func (s *DisputeService) ProcessChargeback(ctx context.Context, webhook ChargebackWebhook) error {
	var actualDisputeData Dispute
	err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txDisputeRepo(tx)

		dispute, err := txRepo.GetDisputeByOrderID(ctx, webhook.OrderID)
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
			created, err := txRepo.CreateDispute(ctx, newDispute)
			if err != nil {
				return fmt.Errorf("create dispute: %w", err)
			}
			actualDisputeData = *created

			return nil
		}

		actualDisputeData, err = ApplyChargebackWebhook(*dispute, webhook)
		if err != nil {
			return err
		}

		if err := txRepo.UpdateDispute(ctx, actualDisputeData); err != nil {
			return fmt.Errorf("update dispute: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("process chargeback: %w", err)
	}

	if err := s.saveWebhookEvent(ctx, actualDisputeData, webhook); err != nil {
		return fmt.Errorf("create dispute event: %w", err)
	}

	return nil
}

func (s *DisputeService) UpsertEvidence(ctx context.Context, disputeID string, upsert EvidenceUpsert) (*Evidence, error) {
	var result *Evidence

	err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txDisputeRepo(tx)

		// 1. Validate that dispute exists and is editable
		dispute, err := txRepo.GetDisputeByID(ctx, disputeID)
		if err != nil {
			return fmt.Errorf("get dispute by id: %w", err)
		}
		if dispute == nil {
			return fmt.Errorf("dispute not found")
		}

		if !IsDisputeEditable(dispute.Status) {
			return fmt.Errorf("dispute cannot be edited in status: %s", dispute.Status)
		}

		//TODO: https://api-docs.solidgate.com/#tag/Files-upload/operation/create-upload-url
		// upload file to Silvergate -- get file id
		// set external file id
		// save files

		// 2. Upsert evidence
		evidence, err := txRepo.UpsertEvidence(ctx, disputeID, upsert)
		if err != nil {
			return fmt.Errorf("upsert evidence: %w", err)
		}
		result = evidence

		// 3. Update dispute status from open to under_review if needed
		if dispute.Status == DisputeOpen {
			dispute.Status = DisputeUnderReview
			if err := txRepo.UpdateDispute(ctx, *dispute); err != nil {
				return fmt.Errorf("update dispute status: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("upsert evidence: %w", err)
	}

	// 4. Create evidence_added event
	if err := s.createEvidenceAddedEvent(ctx, disputeID, *result); err != nil {
		return nil, fmt.Errorf("create evidence event: %w", err)
	}

	return result, nil
}

func (s *DisputeService) Submit(ctx context.Context, disputeID string) error {
	var result *gateway.RepresentmentResult
	err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := s.txDisputeRepo(tx)

		d, err := txRepo.GetDisputeByID(ctx, disputeID)
		if err != nil {
			return fmt.Errorf("get dispute: %w", err)
		}
		if d == nil {
			return fmt.Errorf("dispute not found")
		}

		//TODO: refactor
		if d.Status != DisputeOpen && d.Status != DisputeUnderReview {
			return fmt.Errorf("status %s not eligible for submission", d.Status)
		}

		//TODO: lock evidence before submit
		evidence, err := txRepo.GetEvidence(ctx, disputeID)
		if err != nil {
			return fmt.Errorf("get evidence : %w", err)
		}

		if evidence == nil {
			return fmt.Errorf("evidence not found")
		}

		// TODO: do call out of tx
		// TODO: add retry, idempotency
		res, err := s.provider.SubmitRepresentment(ctx, gateway.RepresentmentRequest{
			OrderId:  d.OrderID,
			Evidence: evidence.Evidence,
		})
		if err != nil {
			return fmt.Errorf("provider submit: %w", err)
		}
		result = &res

		slog.DebugContext(ctx, "Submit representment response",
			"provider_submission_id", res.ProviderSubmissionID)

		//TODO: refactor
		d.SubmittedAt = pointers.Ptr(time.Now())
		d.SubmittingId = pointers.Ptr(res.ProviderSubmissionID)
		d.Status = DisputeSubmitted

		err = txRepo.UpdateDispute(ctx, *d)
		if err != nil {
			return fmt.Errorf("update dispute: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("submit evidence: %w", err)
	}

	err = s.createEvidenceSubmittedEvent(ctx, disputeID, result)
	if err != nil {
		return fmt.Errorf("create submitted event: %w", err)
	}
	return nil

}

func (s *DisputeService) createEvidenceSubmittedEvent(ctx context.Context, disputeID string, result *gateway.RepresentmentResult) error {
	payload, _ := json.Marshal(result)

	disputeEvent := NewDisputeEvent{
		DisputeID:       disputeID,
		Kind:            DisputeEventEvidenceSubmitted,
		ProviderEventID: result.ProviderSubmissionID,
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	if _, err := s.disputeEvents.CreateDisputeEvent(ctx, disputeEvent); err != nil {
		return fmt.Errorf("create dispute event: %w", err)
	}
	return nil
}

func (s *DisputeService) createEvidenceAddedEvent(ctx context.Context, disputeID string, evidence Evidence) error {
	payload, _ := json.Marshal(evidence)

	disputeEvent := NewDisputeEvent{
		DisputeID:       disputeID,
		Kind:            DisputeEventEvidenceAdded,
		ProviderEventID: uuid.New().String(), // Generate unique ID for internal events
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	if _, err := s.disputeEvents.CreateDisputeEvent(ctx, disputeEvent); err != nil {
		return fmt.Errorf("create dispute event: %w", err)
	}
	return nil
}

func (s *DisputeService) saveWebhookEvent(ctx context.Context, dispute Dispute, webhook ChargebackWebhook) error {
	payload, _ := json.Marshal(webhook)

	disputeEvent := NewDisputeEvent{
		DisputeID:       dispute.ID,
		Kind:            deriveKindFromChargebackStatus(webhook.Status),
		ProviderEventID: webhook.ProviderEventID,
		Data:            payload,
		CreatedAt:       time.Now(),
	}

	if _, err := s.disputeEvents.CreateDisputeEvent(ctx, disputeEvent); err != nil {
		return fmt.Errorf("create dispute event: %w", err)
	}
	return nil
}
