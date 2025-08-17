package dispute

import "time"

type Evidence struct {
	DisputeID string `json:"dispute_id"`
	EvidenceUpsert
	UpdatedAt time.Time `json:"updated_at"`
}

type EvidenceUpsert struct {
	Fields map[string]string `json:"fields"`
	Files  []EvidenceFile    `json:"files"`
}

type EvidenceFile struct {
	FileID      string `json:"file_id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}
