package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"TestTaskJustPay/services/paymanager/domain/payment"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service *payment.PaymentService
}

func NewPaymentHandler(s *payment.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: s}
}

func (h *PaymentHandler) Create(c *gin.Context) {
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

func (h *PaymentHandler) Get(c *gin.Context) {
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

func (h *PaymentHandler) Void(c *gin.Context) {
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
