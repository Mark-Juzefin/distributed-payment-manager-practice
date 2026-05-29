package ordercontroller

import (
	"errors"
	"fmt"
	"net/http"

	"TestTaskJustPay/services/paymanager/internal/order"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	service *order.OrderService
}

func NewHTTPHandler(s *order.OrderService) *HTTPHandler {
	return &HTTPHandler{service: s}
}

func (h *HTTPHandler) Get(c *gin.Context) {
	orderID := c.Param("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing order_id"})
		return
	}
	fmt.Println("get orderID:", orderID)
	res, err := h.service.GetOrderByID(c, orderID)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HTTPHandler) GetEvents(c *gin.Context) {
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

func (h *HTTPHandler) Filter(c *gin.Context) {
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

func (h *HTTPHandler) Hold(c *gin.Context) {
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
		if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *HTTPHandler) Capture(c *gin.Context) {
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
		if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if errors.Is(err, order.ErrOnHold) || errors.Is(err, order.ErrInFinalStatus) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *HTTPHandler) createFilter(c *gin.Context) (*order.OrdersQuery, error) {
	query, err := order.NewOrdersQueryBuilder().Build()
	if err != nil {
		return nil, fmt.Errorf("invalid filter params: %w", err)
	}

	return query, nil
}
