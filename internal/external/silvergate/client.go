package silvergate

import (
	"TestTaskJustPay/internal/domain/gateway"
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
	HTTP                   *http.Client
}

func New(baseURL string, submitRepresentmentPath string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		BaseURL:                baseURL,
		SubmitRepresentmentUrl: baseURL + submitRepresentmentPath,
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
