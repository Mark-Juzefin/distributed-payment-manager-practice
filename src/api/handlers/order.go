package handlers

import (
	"TestTaskJustPay/src/service"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
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

//func (h *OrderHandler) GetAll(c *gin.Context) {
//	c.String(http.StatusOK, "GET ALL")
//}
