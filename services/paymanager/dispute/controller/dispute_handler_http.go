package controller

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/paymanager/dispute"
	"TestTaskJustPay/services/paymanager/order"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *dispute.DisputeService
}

func NewHTTPHandler(s *dispute.DisputeService) *HTTPHandler {
	return &HTTPHandler{service: s}
}

func (h *HTTPHandler) GetDisputes(c *gin.Context) {
	disputes, err := h.service.GetDisputes(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, disputes)
}

func (h *HTTPHandler) GetDispute(c *gin.Context) {
	disputeID := c.Param("dispute_id")
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing dispute_id"})
		return
	}

	d, err := h.service.GetDisputeByID(c, disputeID)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Dispute not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, d)
}

func (h *HTTPHandler) Submit(c *gin.Context) {
	disputeID := c.Param("dispute_id")
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "dispute_id is required"})
		return
	}

	err := h.service.Submit(c, disputeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *HTTPHandler) UpsertEvidence(c *gin.Context) {
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

func (h *HTTPHandler) GetEvents(c *gin.Context) {
	var query dispute.DisputeEventQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	res, err := h.service.GetEvents(c, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HTTPHandler) GetEvidence(c *gin.Context) {
	disputeID := c.Param("dispute_id")
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing dispute_id"})
		return
	}

	evidence, err := h.service.GetEvidence(c, disputeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	if evidence == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Evidence not found"})
		return
	}

	c.JSON(http.StatusOK, evidence)
}
