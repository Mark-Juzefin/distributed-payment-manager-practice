package dto

type PaymentWebhookRequest struct {
	Event         string `json:"event"`
	TransactionID string `json:"transaction_id"`
	OrderID       string `json:"order_id"`
	MerchantID    string `json:"merchant_id"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Timestamp     string `json:"timestamp"`
}
