package dispute

import (
	"TestTaskJustPay/services/paymanager/gateway"
	"time"
)

type Evidence struct {
	DisputeID string `json:"dispute_id"`
	gateway.Evidence
	UpdatedAt time.Time `json:"updated_at"`
}

type EvidenceUpsert struct {
	gateway.Evidence
}
