package silvergate

import (
	"TestTaskJustPay/internal/shared/domain/gateway"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL                string
	SubmitRepresentmentUrl string
	CaptureUrl             string
	HTTP                   *http.Client
}

func New(baseURL string, submitRepresentmentPath, capturePath string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		BaseURL:                baseURL,
		SubmitRepresentmentUrl: baseURL + submitRepresentmentPath,
		CaptureUrl:             baseURL + capturePath,
		HTTP:                   httpClient,
	}
}

type createReq struct {
	OrderId         string   `json:"order_id"`
	EvidencesFileID []string `json:"evidences_file_id,omitempty"`
}

type createResp struct {
	ID string `json:"id,omitempty"`
}

type captureReq struct {
	OrderID        string  `json:"order_id"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	IdempotencyKey string  `json:"idempotency_key"`
}

type captureResp struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

// todo: Integration tests with wiremock: validation, timeout
func (c *Client) SubmitRepresentment(ctx context.Context, req gateway.RepresentmentRequest) (gateway.RepresentmentResult, error) {
	body := createReq{
		OrderId:         req.OrderId,
		EvidencesFileID: nil,
	}

	body.EvidencesFileID = make([]string, 0, len(req.Evidence.Files))
	for _, f := range req.Evidence.Files {
		body.EvidencesFileID = append(body.EvidencesFileID, f.FileID)
	}

	j, err := json.Marshal(body)
	if err != nil {
		return gateway.RepresentmentResult{
			ProviderSubmissionID: "",
		}, fmt.Errorf("marshal: %w", err)
	}

	httpReq, _ := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.SubmitRepresentmentUrl,
		bytes.NewReader(j),
	)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return gateway.RepresentmentResult{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode/100 != 2 {
		return gateway.RepresentmentResult{
			ProviderSubmissionID: "",
		}, fmt.Errorf("provider %s: %s", resp.Status, string(raw))
	}

	var out createResp
	_ = json.Unmarshal(raw, &out)

	return gateway.RepresentmentResult{
		ProviderSubmissionID: out.ID,
	}, nil
}

func (c *Client) CapturePayment(ctx context.Context, req gateway.CaptureRequest) (gateway.CaptureResult, error) {
	body := captureReq{
		OrderID:        req.OrderID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		IdempotencyKey: req.IdempotencyKey,
	}

	j, err := json.Marshal(body)
	if err != nil {
		return gateway.CaptureResult{}, fmt.Errorf("marshal capture request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.CaptureUrl,
		bytes.NewReader(j),
	)
	if err != nil {
		return gateway.CaptureResult{}, fmt.Errorf("create capture request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return gateway.CaptureResult{}, fmt.Errorf("http capture request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode/100 != 2 {
		return gateway.CaptureResult{
			Status: gateway.CaptureStatusFailed,
		}, fmt.Errorf("capture provider %s: %s", resp.Status, string(raw))
	}

	var out captureResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return gateway.CaptureResult{}, fmt.Errorf("unmarshal capture response: %w", err)
	}

	status := gateway.CaptureStatusFailed
	if out.Status == "success" {
		status = gateway.CaptureStatusSuccess
	}

	return gateway.CaptureResult{
		ProviderTxID: out.TransactionID,
		Status:       status,
	}, nil
}
