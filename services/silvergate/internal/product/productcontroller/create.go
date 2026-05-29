package productcontroller

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/silvergate/internal/merchantauth"
	"TestTaskJustPay/services/silvergate/internal/product"

	"github.com/gin-gonic/gin"
)

type createProductRequest struct {
	Slug        *string `json:"slug,omitempty"`
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       int64   `json:"price" binding:"required,min=1"`
	Currency    string  `json:"currency" binding:"required,len=3"`
}

type CreateHandler struct {
	svc *product.Service
}

func NewCreateHandler(svc *product.Service) *CreateHandler {
	return &CreateHandler{svc: svc}
}

func (h *CreateHandler) Handle(c *gin.Context) {
	merchantID, ok := merchantauth.FromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "merchant context missing"})
		return
	}

	var req createProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	p, err := h.svc.Create(c.Request.Context(), merchantID, product.CreateInput{
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Currency:    req.Currency,
	})
	if err != nil {
		writeCreateError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toProductResponse(p))
}

func writeCreateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, product.ErrInvalidSlug):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
	case errors.Is(err, product.ErrSlugConflict):
		c.JSON(http.StatusConflict, errorResponse{Error: err.Error(), Code: "slug_conflict"})
	default:
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}
