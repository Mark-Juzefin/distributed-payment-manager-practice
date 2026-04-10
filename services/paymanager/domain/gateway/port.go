package gateway

import "context"

//go:generate mockgen -source port.go -destination mock_port.go -package gateway

// TODO: Add domain errors and return them from SubmitRepresentment. Describe each here
type Provider interface {
	SubmitRepresentment(ctx context.Context, req RepresentmentRequest) (RepresentmentResult, error)
	CapturePayment(ctx context.Context, req CaptureRequest) (CaptureResult, error)
	AuthorizePayment(ctx context.Context, req AuthRequest) (AuthResult, error)
	VoidPayment(ctx context.Context, req VoidRequest) (VoidResult, error)
}

type RepresentmentRequest struct {
	OrderId string
	Evidence
	// TODO: додати IdempotencyKey для безпечних ретраїв

}

type Evidence struct {
	Fields map[string]string `json:"fields"`
	Files  []EvidenceFile    `json:"files"`
}

type EvidenceFile struct {
	FileID      string `json:"file_id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

type RepresentmentResult struct {
	ProviderSubmissionID string
}

type CaptureRequest struct {
	OrderID        string
	Amount         float64
	Currency       string
	IdempotencyKey string
}

type CaptureResult struct {
	ProviderTxID string
	Status       CaptureStatus
}

type AuthRequest struct {
	MerchantID string
	OrderID    string // payment ID from merchant perspective
	Amount     int64
	Currency   string
	CardToken  string
}

type AuthResult struct {
	TransactionID string
	Status        AuthStatus
	DeclineReason string
}

type AuthStatus string

const (
	AuthStatusAuthorized AuthStatus = "authorized"
	AuthStatusDeclined   AuthStatus = "declined"
)

type VoidRequest struct {
	TransactionID string
}

type VoidResult struct {
	TransactionID string
	Status        string
}

type CaptureStatus string

const (
	CaptureStatusSuccess CaptureStatus = "success"
	CaptureStatusFailed  CaptureStatus = "failed"
)
