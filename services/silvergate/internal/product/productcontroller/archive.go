package productcontroller

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/silvergate/internal/merchantauth"
	"TestTaskJustPay/services/silvergate/internal/product"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ArchiveHandler struct {
	svc *product.Service
}

func NewArchiveHandler(svc *product.Service) *ArchiveHandler {
	return &ArchiveHandler{svc: svc}
}

func (h *ArchiveHandler) Handle(c *gin.Context) {
	merchantID, id, ok := resolveProductIdentity(c)
	if !ok {
		return
	}

	if err := h.svc.Archive(c.Request.Context(), merchantID, id); err != nil {
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

func resolveProductIdentity(c *gin.Context) (string, uuid.UUID, bool) {
	merchantID, ok := merchantauth.FromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "merchant context missing"})
		return "", uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid product id"})
		return "", uuid.Nil, false
	}
	return merchantID, id, true
}

func writeStatusError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, product.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}
