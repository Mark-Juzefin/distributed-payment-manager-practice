package handlers

import (
	"TestTaskJustPay/internal/domain/dispute"
	"net/http"

	"github.com/gin-gonic/gin"
)

type DisputeHandler struct {
	service *dispute.DisputeService
}

func NewDisputeHandler(s *dispute.DisputeService) DisputeHandler {
	return DisputeHandler{service: s}
}

func (h *DisputeHandler) GetDisputes(c *gin.Context) {
	disputes, err := h.service.GetDisputes(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, disputes)
}
