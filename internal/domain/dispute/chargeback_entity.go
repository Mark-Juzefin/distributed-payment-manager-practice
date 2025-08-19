package dispute

import (
	"strings"
	"time"
)

type ChargebackWebhook struct {
	DisputeID string           `json:"dispute_id"`
	OrderID   string           `json:"order_id"`
	Status    ChargebackStatus `json:"status"`
	Reason    string           `json:"reason"`
	Money
	OccurredAt    time.Time         `json:"occurred_at"`
	EvidenceDueAt *time.Time        `json:"evidence_due_at,omitempty"`
	Meta          map[string]string `json:"meta"`
}

type ChargebackStatus string

const (
	ChargebackOpened  ChargebackStatus = "opened"
	ChargebackUpdated ChargebackStatus = "updated"
	ChargebackClosed  ChargebackStatus = "closed"
)

type ChargebackResolution string

const (
	ChargebackResolutionWon  ChargebackResolution = "won"
	ChargebackResolutionLost ChargebackResolution = "lost"
)

func (e ChargebackWebhook) Resolution() (ChargebackResolution, bool) {
	if e.Meta == nil {
		return "", false
	}
	v := strings.ToLower(e.Meta["resolution"])
	switch v {
	case "won":
		return ChargebackResolutionWon, true
	case "lost":
		return ChargebackResolutionLost, true
	default:
		return "", false
	}
}
