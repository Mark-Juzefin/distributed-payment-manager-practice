package dispute

import (
	"fmt"
	"time"
)

type Dispute struct {
	ID     string        `json:"dispute_id"`
	Status DisputeStatus `json:"status"`
	DisputeInfo
}

type NewDispute struct {
	Status DisputeStatus
	DisputeInfo
}

type DisputeInfo struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
	Money
	OpenedAt      time.Time  `json:"opened_at"`
	EvidenceDueAt *time.Time `json:"evidence_due_at,omitempty"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
}

type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type DisputeStatus string

const (
	DisputeOpen        DisputeStatus = "open"
	DisputeUnderReview DisputeStatus = "under_review"
	DisputeSubmitted   DisputeStatus = "submitted"
	DisputeWon         DisputeStatus = "won"
	DisputeLost        DisputeStatus = "lost"
	DisputeClosed      DisputeStatus = "closed" // means the chargeback was closet but the result (won/lost) wasn't specified
	DisputeCanceled    DisputeStatus = "canceled"
)

func ApplyChargebackWebhook(dispute Dispute, ev ChargebackWebhook) (Dispute, error) {
	status, closedAt, err := resolveChargebackStatus(ev)
	if err != nil {
		return dispute, fmt.Errorf("convert chargeback status: %w", err)
	}

	// Only overwrite dispute.Status for non-Updated events
	if ev.Status != ChargebackUpdated {
		dispute.Status = status
	}

	if ev.EvidenceDueAt != nil {
		dispute.EvidenceDueAt = ev.EvidenceDueAt
	}
	if closedAt != nil {
		dispute.ClosedAt = closedAt
	}

	return dispute, nil
}

func resolveChargebackStatus(w ChargebackWebhook) (DisputeStatus, *time.Time, error) {
	switch w.Status {
	case ChargebackOpened:
		return DisputeOpen, nil, nil
	case ChargebackUpdated:
		return DisputeOpen, nil, nil // means evidence window
	case ChargebackClosed:
		if res, ok := w.Resolution(); ok {
			switch res {
			case ChargebackResolutionWon:
				return DisputeWon, &w.OccurredAt, nil
			case ChargebackResolutionLost:
				return DisputeLost, &w.OccurredAt, nil
			default:
				//todo: FUTURE COMPATIBILITY
				return "", nil, fmt.Errorf("unknown resolution: %q", w.Meta["resolution"])
			}
		}
		return DisputeClosed, &w.OccurredAt, nil
	default:
		//todo: FUTURE COMPATIBILITY
		return "", nil, fmt.Errorf("unknown chargeback status: %s", w.Status)
	}
}
