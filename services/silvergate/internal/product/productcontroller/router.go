package productcontroller

import (
	"TestTaskJustPay/services/silvergate/internal/product"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes constructs all product handlers from svc and mounts them on rg.
// Callers wire the merchant-auth middleware on rg before calling this.
func RegisterRoutes(rg *gin.RouterGroup, svc *product.Service) {
	create := NewCreateHandler(svc)
	get := NewGetHandler(svc)
	list := NewListHandler(svc)
	update := NewUpdateHandler(svc)
	archive := NewArchiveHandler(svc)
	unarchive := NewUnarchiveHandler(svc)

	rg.POST("", create.Handle)
	rg.GET("", list.Handle)
	rg.GET("/:id", get.Handle)
	rg.PATCH("/:id", update.Handle)
	rg.POST("/:id/archive", archive.Handle)
	rg.POST("/:id/unarchive", unarchive.Handle)
}
