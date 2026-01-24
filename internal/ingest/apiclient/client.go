package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"TestTaskJustPay/internal/shared/dto"
)

// Client defines the interface for API service client.
// This allows for different implementations (HTTP, gRPC).
type Client interface {
	SendOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error
	SendDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error
	Close() error
}

// HTTPClient implements Client using HTTP.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
	retryCfg   RetryConfig
}

// HTTPClientConfig holds configuration for HTTPClient.
type HTTPClientConfig struct {
	BaseURL        string
	Timeout        time.Duration
	RetryAttempts  int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
}

// NewHTTPClient creates a new HTTP client for API service.
func NewHTTPClient(cfg HTTPClientConfig) *HTTPClient {
	return &HTTPClient{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		retryCfg: RetryConfig{
			MaxAttempts: cfg.RetryAttempts,
			BaseDelay:   cfg.RetryBaseDelay,
			MaxDelay:    cfg.RetryMaxDelay,
		},
	}
}

// SendOrderUpdate sends an order update to the API service.
func (c *HTTPClient) SendOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error {
	return DoWithRetry(ctx, c.retryCfg, func() error {
		return c.sendRequest(ctx, "/internal/updates/orders", req)
	})
}

// SendDisputeUpdate sends a dispute update to the API service.
func (c *HTTPClient) SendDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error {
	return DoWithRetry(ctx, c.retryCfg, func() error {
		return c.sendRequest(ctx, "/internal/updates/disputes", req)
	})
}

// Close releases any resources held by the client.
func (c *HTTPClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

func (c *HTTPClient) sendRequest(ctx context.Context, path string, body any) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrServiceUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp)
}

func (c *HTTPClient) handleResponse(resp *http.Response) error {
	// Read response body for error messages
	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrBadRequest, string(body))
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode == http.StatusConflict:
		return ErrConflict
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: %s", ErrInvalidStatus, string(body))
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: status %d, body: %s", ErrServiceUnavailable, resp.StatusCode, string(body))
	default:
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}
}
