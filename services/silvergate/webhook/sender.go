package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"TestTaskJustPay/services/silvergate/domain/transaction"
)

type Event struct {
	Event         string `json:"event"`
	TransactionID string `json:"transaction_id"`
	RefundID      string `json:"refund_id,omitempty"`
	OrderID       string `json:"order_id"`
	MerchantID    string `json:"merchant_id"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Timestamp     string `json:"timestamp"`
}

type Sender struct {
	callbackURL string
	client      *http.Client
	log         *slog.Logger
}

func NewSender(callbackURL string, log *slog.Logger) *Sender {
	return &Sender{
		callbackURL: callbackURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		log: log,
	}
}

func (s *Sender) SendCaptureResult(ctx context.Context, tx *transaction.Transaction) error {
	var eventName string
	switch tx.Status {
	case transaction.StatusCaptured:
		eventName = "transaction.captured"
	case transaction.StatusCaptureFailed:
		eventName = "transaction.capture_failed"
	case transaction.StatusVoided:
		eventName = "transaction.voided"
	default:
		eventName = "transaction." + string(tx.Status)
	}

	return s.sendEvent(ctx, Event{
		Event:         eventName,
		TransactionID: tx.ID.String(),
		OrderID:       tx.OrderRef,
		MerchantID:    tx.MerchantID,
		Status:        string(tx.Status),
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Sender) SendRefundResult(ctx context.Context, tx *transaction.Transaction, refund *transaction.Refund) error {
	eventName := "transaction.refunded"
	if refund.Status == transaction.RefundStatusFailed {
		eventName = "transaction.refund_failed"
	}

	evt := Event{
		Event:         eventName,
		TransactionID: tx.ID.String(),
		RefundID:      refund.ID.String(),
		OrderID:       tx.OrderRef,
		MerchantID:    tx.MerchantID,
		Status:        string(refund.Status),
		Amount:        refund.Amount,
		Currency:      tx.Currency,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	return s.sendEvent(ctx, evt)
}

func (s *Sender) sendEvent(ctx context.Context, evt Event) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal webhook: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.callbackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("webhook rejected: status %d", resp.StatusCode)
	}

	s.log.Info("webhook sent",
		"event", evt.Event,
		"transaction_id", evt.TransactionID,
		"callback_url", s.callbackURL,
	)

	return nil
}
