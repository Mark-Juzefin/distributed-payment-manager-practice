package handlers

import (
	"TestTaskJustPay/src/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

type OrderHandler struct {
	Service service.IOrderService
}

func NewOrderHandler(s service.IOrderService) OrderHandler {
	return OrderHandler{Service: s}
}

func (h *OrderHandler) Get(c *gin.Context) {
	c.String(http.StatusOK, "GET")
}

func (h *OrderHandler) GetAll(c *gin.Context) {
	c.String(http.StatusOK, "GET ALL")
}
