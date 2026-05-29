package purchasecontroller

import (
	"TestTaskJustPay/services/silvergate/internal/purchase"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts POST /purchase under rg. Callers wire merchant-auth on
// rg before calling.
func RegisterRoutes(rg *gin.RouterGroup, svc *purchase.Service) {
	h := NewHandler(svc)
	rg.POST("", h.Handle)
}
