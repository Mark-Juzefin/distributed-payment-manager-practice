package handlers

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/webhook"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	service   *order.OrderService
	processor webhook.Processor
}

func NewOrderHandler(s *order.OrderService, processor webhook.Processor) OrderHandler {
	return OrderHandler{service: s, processor: processor}
}

func (h *OrderHandler) Webhook(c *gin.Context) {
	var event order.PaymentWebhook
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing order_id"})
		return
	}

	err := h.processor.ProcessOrderWebhook(c.Request.Context(), event)
	if err != nil {
		if errors.Is(err, apperror.ErrUnappropriatedStatus) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		} else if errors.Is(err, apperror.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		} else if errors.Is(err, apperror.ErrEventAlreadyStored) {
			c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}
		return
	}

	c.Status(http.StatusAccepted)
}

func (h *OrderHandler) Get(c *gin.Context) {
	orderID := c.Param("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing order_id"})
		return
	}
	fmt.Println("get orderID:", orderID)
	res, err := h.service.GetOrderByID(c, orderID)
	if err != nil {
		if errors.Is(err, apperror.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
	}

	c.JSON(http.StatusOK, res)
}
func (h *OrderHandler) GetEvents(c *gin.Context) {
	var query order.OrderEventQuery
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

func (h *OrderHandler) Filter(c *gin.Context) {
	filter, err := h.createFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.service.GetOrders(c, *filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// todo: redo
type FilterParams struct {
	StatusArr  string `form:"status"`  // todo: redo
	UserID     string `form:"user_id"` // todo: redo
	PageSize   int    `form:"limit" binding:"omitempty,min=0" default:"10"`
	PageNumber int    `form:"offset" binding:"omitempty,min=0" default:"0"`
	SortBy     string `form:"sort_by" binding:"omitempty,oneof=created_at updated_at" default:"created_at"`
	SortOrder  string `form:"sort_order" binding:"omitempty,oneof=asc desc" default:"desc"`
}

func (h *OrderHandler) Hold(c *gin.Context) {
	orderID := c.Param("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing order_id"})
		return
	}

	var request order.HoldRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	response, err := h.service.UpdateOrderHold(c, orderID, request)
	if err != nil {
		if errors.Is(err, apperror.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) Capture(c *gin.Context) {
	orderID := c.Param("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing order_id"})
		return
	}

	var request order.CaptureRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	response, err := h.service.CapturePayment(c, orderID, request)
	if err != nil {
		if errors.Is(err, apperror.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if errors.Is(err, apperror.ErrOrderOnHold) || errors.Is(err, apperror.ErrOrderInFinalStatus) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *OrderHandler) createFilter(c *gin.Context) (*order.OrdersQuery, error) {
	//var params FilterParams

	//if err := c.ShouldBindQuery(&params); err != nil {
	//	return nil, err
	//}
	//
	//statusArr := strings.Split(params.StatusArr, ",")
	//
	//statuses := make([]order.Status, len(statusArr))
	//for i, v := range statusArr {
	//	s, err := order.NewStatus(v)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	statuses[i] = s
	//}
	//
	//if params.PageSize == 0 {
	//	params.PageSize = 10
	//}
	//if params.SortBy == "" {
	//	params.SortBy = "created_at"
	//}
	//if params.SortOrder == "" {
	//	params.SortOrder = "desc"
	//}

	//query, err := order.NewOrdersQueryBuilder().
	//	WithPagination(order.Pagination{
	//		PageSize:   params.PageSize,
	//		PageNumber: params.PageNumber,
	//	}).WithSort(params.SortBy, params.SortOrder).Build()

	query, err := order.NewOrdersQueryBuilder().Build()
	if err != nil {
		return nil, fmt.Errorf("invalid filter params: %w", err)
	}

	return query, nil
}
