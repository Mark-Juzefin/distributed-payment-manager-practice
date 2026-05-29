package paymentcontroller

import (
	"net/http"

	"TestTaskJustPay/services/paymanager/internal/payment"

	"github.com/gin-gonic/gin"
)

// HandleWebhook processes a payment webhook forwarded from the Ingest service.
func (h *HTTPHandler) HandleWebhook(c *gin.Context) {
	var req payment.CaptureWebhook
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := h.service.ProcessCaptureWebhook(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
