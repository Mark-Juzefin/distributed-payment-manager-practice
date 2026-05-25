package productcontroller

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/silvergate/internal/merchantauth"
	"TestTaskJustPay/services/silvergate/internal/product"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GetHandler struct {
	svc *product.Service
}

func NewGetHandler(svc *product.Service) *GetHandler {
	return &GetHandler{svc: svc}
}

func (h *GetHandler) Handle(c *gin.Context) {
	merchantID, ok := merchantauth.FromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "merchant context missing"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid product id"})
		return
	}

	p, err := h.svc.Get(c.Request.Context(), merchantID, id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			c.JSON(http.StatusNotFound, errorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	c.JSON(http.StatusOK, toProductResponse(p))
}
