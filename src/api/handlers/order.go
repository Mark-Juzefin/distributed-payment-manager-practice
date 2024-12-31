package handlers

import (
	"TestTaskJustPay/src/domain"
	"TestTaskJustPay/src/service"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"strings"
)

type OrderHandler struct {
	service service.IOrderService
}

func NewOrderHandler(s service.IOrderService) OrderHandler {
	return OrderHandler{service: s}
}

func (h *OrderHandler) Get(c *gin.Context) {
	orderID := c.Param("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing order_id"})
		return
	}
	fmt.Println("get orderID:", orderID)
	res, err := h.service.Get(c, orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
	}

	c.JSON(http.StatusOK, res)
}

type FilterParams struct {
	StatusArr string `form:"status" binding:"required"`
	UserID    string `form:"user_id" binding:"required"`
	Limit     int    `form:"limit" binding:"omitempty,min=0" default:"10"`
	Offset    int    `form:"offset" binding:"omitempty,min=0" default:"0"`
	SortBy    string `form:"sort_by" binding:"omitempty,oneof=created_at updated_at" default:"created_at"`
	SortOrder string `form:"sort_order" binding:"omitempty,oneof=asc desc" default:"desc"`
}

func (h *OrderHandler) Filter(c *gin.Context) {
	filter, err := h.createFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.service.Filter(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *OrderHandler) createFilter(c *gin.Context) (domain.Filter, error) {
	var params FilterParams

	if err := c.ShouldBindQuery(&params); err != nil {
		return domain.Filter{}, err
	}

	statusArr := strings.Split(params.StatusArr, ",")

	fmt.Println("statusArr", statusArr)
	status := make([]domain.OrderStatus, len(statusArr))
	for i, v := range statusArr {
		s, err := domain.NewOrderStatus(v)
		if err != nil {
			return domain.Filter{}, err
		}

		status[i] = s
	}

	userID, err := uuid.Parse(params.UserID)
	if err != nil {
		return domain.Filter{}, err
	}

	if params.Limit == 0 {
		params.Limit = 10
	}
	if params.SortBy == "" {
		params.SortBy = "created_at"
	}
	if params.SortOrder == "" {
		params.SortOrder = "desc"
	}

	return domain.NewFilter(status, userID, params.Limit, params.Offset, params.SortBy, params.SortOrder), nil
}
