//go:build !integration

package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"TestTaskJustPay/internal/shared/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_SendOrderUpdate(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/internal/updates/orders", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var req dto.OrderUpdateRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "order-123", req.OrderID)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(dto.OrderUpdateResponse{Success: true})
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPClientConfig{
			BaseURL:        server.URL,
			Timeout:        5 * time.Second,
			RetryAttempts:  1,
			RetryBaseDelay: 10 * time.Millisecond,
			RetryMaxDelay:  100 * time.Millisecond,
		})

		err := client.SendOrderUpdate(context.Background(), dto.OrderUpdateRequest{
			OrderID: "order-123",
			Status:  "created",
		})

		assert.NoError(t, err)
	})

	t.Run("returns ErrNotFound on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPClientConfig{
			BaseURL:       server.URL,
			Timeout:       5 * time.Second,
			RetryAttempts: 1,
		})

		err := client.SendOrderUpdate(context.Background(), dto.OrderUpdateRequest{})

		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("returns ErrInvalidStatus on 422", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message": "invalid status transition"}`))
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPClientConfig{
			BaseURL:       server.URL,
			Timeout:       5 * time.Second,
			RetryAttempts: 1,
		})

		err := client.SendOrderUpdate(context.Background(), dto.OrderUpdateRequest{})

		assert.ErrorIs(t, err, ErrInvalidStatus)
	})

	t.Run("returns ErrServiceUnavailable on 500 and retries", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPClientConfig{
			BaseURL:        server.URL,
			Timeout:        5 * time.Second,
			RetryAttempts:  3,
			RetryBaseDelay: 10 * time.Millisecond,
			RetryMaxDelay:  50 * time.Millisecond,
		})

		err := client.SendOrderUpdate(context.Background(), dto.OrderUpdateRequest{})

		assert.ErrorIs(t, err, ErrServiceUnavailable)
		assert.Equal(t, 3, attempts, "should retry 3 times")
	})

	t.Run("succeeds on retry after temporary failure", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(dto.OrderUpdateResponse{Success: true})
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPClientConfig{
			BaseURL:        server.URL,
			Timeout:        5 * time.Second,
			RetryAttempts:  3,
			RetryBaseDelay: 10 * time.Millisecond,
			RetryMaxDelay:  50 * time.Millisecond,
		})

		err := client.SendOrderUpdate(context.Background(), dto.OrderUpdateRequest{})

		assert.NoError(t, err)
		assert.Equal(t, 2, attempts, "should succeed on second attempt")
	})
}

func TestHTTPClient_SendDisputeUpdate(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/internal/updates/disputes", r.URL.Path)

			var req dto.DisputeUpdateRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "order-456", req.OrderID)
			assert.Equal(t, "opened", req.Status)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(dto.DisputeUpdateResponse{Success: true})
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPClientConfig{
			BaseURL:       server.URL,
			Timeout:       5 * time.Second,
			RetryAttempts: 1,
		})

		err := client.SendDisputeUpdate(context.Background(), dto.DisputeUpdateRequest{
			OrderID: "order-456",
			Status:  "opened",
		})

		assert.NoError(t, err)
	})

	t.Run("returns ErrBadRequest on 400", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message": "missing required field"}`))
		}))
		defer server.Close()

		client := NewHTTPClient(HTTPClientConfig{
			BaseURL:       server.URL,
			Timeout:       5 * time.Second,
			RetryAttempts: 1,
		})

		err := client.SendDisputeUpdate(context.Background(), dto.DisputeUpdateRequest{})

		assert.ErrorIs(t, err, ErrBadRequest)
	})
}

func TestHTTPClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(HTTPClientConfig{
		BaseURL:        server.URL,
		Timeout:        5 * time.Second,
		RetryAttempts:  3,
		RetryBaseDelay: 100 * time.Millisecond,
		RetryMaxDelay:  500 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.SendOrderUpdate(ctx, dto.OrderUpdateRequest{})

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
