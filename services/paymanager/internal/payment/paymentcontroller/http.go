package paymentcontroller

import (
	"errors"
	"log/slog"
	"net/http"

	"TestTaskJustPay/services/paymanager/internal/payment"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *payment.PaymentService
}

func NewHTTPHandler(s *payment.PaymentService) *HTTPHandler {
	return &HTTPHandler{service: s}
}

func (h *HTTPHandler) Create(c *gin.Context) {
	var req payment.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.service.CreatePayment(c.Request.Context(), req)
	if err != nil {
		slog.Error("payment creation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "payment creation failed"})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *HTTPHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing payment id"})
		return
	}

	p, err := h.service.GetPaymentByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *HTTPHandler) Void(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing payment id"})
		return
	}

	p, err := h.service.VoidPayment(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
			return
		}
		if errors.Is(err, payment.ErrInvalidStatus) {
			c.JSON(http.StatusConflict, gin.H{"error": "payment cannot be voided in current state"})
			return
		}
		slog.Error("payment void failed", "payment_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "void failed"})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *HTTPHandler) Refund(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing payment id"})
		return
	}

	var req payment.RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p, err := h.service.RefundPayment(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, payment.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
			return
		}
		if errors.Is(err, payment.ErrInvalidStatus) {
			c.JSON(http.StatusConflict, gin.H{"error": "payment cannot be refunded in current state"})
			return
		}
		if errors.Is(err, payment.ErrRefundExceedsAmount) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "refund amount exceeds remaining balance"})
			return
		}
		slog.Error("payment refund failed", "payment_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refund failed"})
		return
	}

	c.JSON(http.StatusAccepted, p)
}
