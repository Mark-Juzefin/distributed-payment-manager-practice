package gateway

// Shared request/response types for the payment provider (Silvergate).
// Provider interfaces are defined in each domain package separately (ISP).

type RepresentmentRequest struct {
	OrderId string
	Evidence
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
	OrderID    string
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

type RefundRequest struct {
	TransactionID  string
	Amount         int64
	IdempotencyKey string
}

type RefundResult struct {
	RefundID      string
	TransactionID string
	Amount        int64
	Status        string
}

type CaptureStatus string

const (
	CaptureStatusSuccess CaptureStatus = "success"
	CaptureStatusFailed  CaptureStatus = "failed"
)
