package payment

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusAuthorized     Status = "authorized"
	StatusDeclined       Status = "declined"
	StatusCapturePending Status = "capture_pending"
	StatusCaptured       Status = "captured"
	StatusCaptureFailed  Status = "capture_failed"
)

var validTransitions = map[Status][]Status{
	StatusAuthorized:     {StatusCapturePending},
	StatusCapturePending: {StatusCaptured, StatusCaptureFailed},
}

func (s Status) CanTransitionTo(target Status) bool {
	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

type Payment struct {
	ID            string    `json:"id"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	CardToken     string    `json:"card_token"`
	Status        Status    `json:"status"`
	DeclineReason string    `json:"decline_reason,omitempty"`
	ProviderTxID  string    `json:"provider_tx_id,omitempty"`
	MerchantID    string    `json:"merchant_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewAuthorized(amount int64, currency, cardToken, providerTxID, merchantID string) Payment {
	now := time.Now().UTC()
	return Payment{
		ID:           uuid.New().String(),
		Amount:       amount,
		Currency:     currency,
		CardToken:    cardToken,
		Status:       StatusAuthorized,
		ProviderTxID: providerTxID,
		MerchantID:   merchantID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func NewDeclined(amount int64, currency, cardToken, providerTxID, merchantID, reason string) Payment {
	now := time.Now().UTC()
	return Payment{
		ID:            uuid.New().String(),
		Amount:        amount,
		Currency:      currency,
		CardToken:     cardToken,
		Status:        StatusDeclined,
		DeclineReason: reason,
		ProviderTxID:  providerTxID,
		MerchantID:    merchantID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

type CaptureWebhook struct {
	Event         string `json:"event"`
	TransactionID string `json:"transaction_id"`
	OrderID       string `json:"order_id"`
	MerchantID    string `json:"merchant_id"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Timestamp     string `json:"timestamp"`
}

type CreatePaymentRequest struct {
	Amount    int64  `json:"amount" binding:"required,min=1"`
	Currency  string `json:"currency" binding:"required,len=3"`
	CardToken string `json:"card_token" binding:"required"`
}
