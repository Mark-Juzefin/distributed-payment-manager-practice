package dispute

import (
	"TestTaskJustPay/internal/api/domain/gateway"
	"time"
)

type Evidence struct { //TODO: rename
	DisputeID string `json:"dispute_id"`
	gateway.Evidence
	UpdatedAt time.Time `json:"updated_at"`
}

type EvidenceUpsert struct {
	gateway.Evidence
}
