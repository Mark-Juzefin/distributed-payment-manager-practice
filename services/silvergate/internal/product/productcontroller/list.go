package productcontroller

import (
	"errors"
	"net/http"
	"strconv"

	"TestTaskJustPay/services/silvergate/internal/merchantauth"
	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/product/productrepo"

	"github.com/gin-gonic/gin"
)

type listResponse struct {
	Items      []productResponse `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

type ListHandler struct {
	svc *product.Service
}

func NewListHandler(svc *product.Service) *ListHandler {
	return &ListHandler{svc: svc}
}

func (h *ListHandler) Handle(c *gin.Context) {
	merchantID, ok := merchantauth.FromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "merchant context missing"})
		return
	}

	filter, err := parseListFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	items, next, err := h.svc.List(c.Request.Context(), merchantID, filter)
	if err != nil {
		if errors.Is(err, product.ErrLimitTooLarge) {
			c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	resp := listResponse{Items: make([]productResponse, 0, len(items))}
	for _, p := range items {
		resp.Items = append(resp.Items, toProductResponse(p))
	}
	if next != nil {
		token := productrepo.EncodeCursor(*next)
		resp.NextCursor = &token
	}
	c.JSON(http.StatusOK, resp)
}

func parseListFilter(c *gin.Context) (product.ListFilter, error) {
	var f product.ListFilter

	switch s := c.Query("status"); s {
	case "":
		// no filter
	case string(product.StatusActive):
		st := product.StatusActive
		f.StatusFilter = &st
	case string(product.StatusArchived):
		st := product.StatusArchived
		f.StatusFilter = &st
	default:
		return f, errors.New("invalid status filter")
	}

	if raw := c.Query("cursor"); raw != "" {
		cur, err := productrepo.DecodeCursor(raw)
		if err != nil {
			return f, errors.New("invalid cursor")
		}
		f.Cursor = cur
	}

	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return f, errors.New("invalid limit")
		}
		f.Limit = n
	}

	return f, nil
}
