package productcontroller

import (
	"errors"
	"net/http"
	"time"

	"TestTaskJustPay/services/silvergate/internal/merchantauth"
	"TestTaskJustPay/services/silvergate/internal/product"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// updateProductRequest is the wire form of a PATCH payload. Nil pointer == "no change".
// "absent" and "null" are deliberately collapsed to the same semantic per spec decision #13.
type updateProductRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Slug        *string `json:"slug,omitempty"`
	Price       *int64  `json:"price,omitempty"`
	Currency    *string `json:"currency,omitempty"`
}

type productResponse struct {
	ID               uuid.UUID  `json:"id"`
	MerchantID       string     `json:"merchant_id"`
	Slug             *string    `json:"slug"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	Price            int64      `json:"price"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	FirstPurchasedAt *time.Time `json:"first_purchased_at"`
	LockedFields     []string   `json:"locked_fields"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type errorResponse struct {
	Error  string   `json:"error"`
	Code   string   `json:"code,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

type UpdateHandler struct {
	svc *product.Service
}

func NewUpdateHandler(svc *product.Service) *UpdateHandler {
	return &UpdateHandler{svc: svc}
}

func (h *UpdateHandler) Handle(c *gin.Context) {
	merchantID, ok := merchantauth.FromContext(c.Request.Context())
	if !ok {
		// Middleware should have caught this; defensive 401 for direct misuse.
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "merchant context missing"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid product id"})
		return
	}

	var req updateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	updated, err := h.svc.Update(c.Request.Context(), merchantID, id, product.UpdateRequest{
		Name:        req.Name,
		Description: req.Description,
		Slug:        req.Slug,
		Price:       req.Price,
		Currency:    req.Currency,
	})
	if err != nil {
		writeUpdateError(c, err)
		return
	}

	c.JSON(http.StatusOK, toProductResponse(updated))
}

func writeUpdateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, product.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: err.Error()})
	case errors.Is(err, product.ErrSlugConflict):
		c.JSON(http.StatusConflict, errorResponse{Error: err.Error(), Code: "slug_conflict"})
	case errors.Is(err, product.ErrFieldsLocked):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{
			Error:  err.Error(),
			Code:   "fields_locked",
			Fields: product.LockedAfterPurchase{}.FieldNames(),
		})
	case errors.Is(err, product.ErrArchived):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Error: err.Error(), Code: "product_archived"})
	case errors.Is(err, product.ErrInvalidSlug),
		errors.Is(err, product.ErrSlugRemoval),
		errors.Is(err, product.ErrEmptyUpdate):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}

func toProductResponse(p *product.Product) productResponse {
	locked := p.LockedFields()
	if locked == nil {
		locked = []string{}
	}
	return productResponse{
		ID:               p.ID,
		MerchantID:       p.MerchantID,
		Slug:             p.Slug,
		Name:             p.Name,
		Description:      p.Description,
		Price:            p.Price,
		Currency:         p.Currency,
		Status:           string(p.Status),
		FirstPurchasedAt: p.FirstPurchasedAt,
		LockedFields:     locked,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}
