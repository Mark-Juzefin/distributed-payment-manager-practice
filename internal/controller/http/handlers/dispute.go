package handlers

import (
	"TestTaskJustPay/internal/domain/dispute"
	"fmt"
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

func (h *DisputeHandler) UpsertEvidence(c *gin.Context) {
	disputeID := c.Param("dispute_id")
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "dispute_id is required"})
		return
	}

	var upsert dispute.EvidenceUpsert
	if err := c.ShouldBindJSON(&upsert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	evidence, err := h.service.UpsertEvidence(c, disputeID, upsert)
	if err != nil {
		if err.Error() == "dispute not found" {
			c.JSON(http.StatusNotFound, gin.H{"message": "Dispute not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, evidence)
}

func (h *DisputeHandler) GetEvents(c *gin.Context) {
	disputeID := c.Param("dispute_id")
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing dispute_id"})
		return
	}
	fmt.Println("get disputeID:", disputeID)
	res, err := h.service.GetEvents(c, disputeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
	}

	c.JSON(http.StatusOK, res)
}
