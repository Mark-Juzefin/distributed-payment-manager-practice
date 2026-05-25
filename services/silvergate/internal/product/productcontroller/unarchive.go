package productcontroller

import (
	"net/http"

	"TestTaskJustPay/services/silvergate/internal/product"

	"github.com/gin-gonic/gin"
)

type UnarchiveHandler struct {
	svc *product.Service
}

func NewUnarchiveHandler(svc *product.Service) *UnarchiveHandler {
	return &UnarchiveHandler{svc: svc}
}

func (h *UnarchiveHandler) Handle(c *gin.Context) {
	merchantID, id, ok := resolveProductIdentity(c)
	if !ok {
		return
	}

	if err := h.svc.Unarchive(c.Request.Context(), merchantID, id); err != nil {
		writeStatusError(c, err)
		return
	}

	p, err := h.svc.Get(c.Request.Context(), merchantID, id)
	if err != nil {
		writeStatusError(c, err)
		return
	}
	c.JSON(http.StatusOK, toProductResponse(p))
}
