package silvergate

import (
	"TestTaskJustPay/services/silvergate/internal/merchantauth"
	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/product/productcontroller"
	"TestTaskJustPay/services/silvergate/internal/purchase"
	"TestTaskJustPay/services/silvergate/internal/purchase/purchasecontroller"
	"TestTaskJustPay/services/silvergate/internal/transaction/transactioncontroller"

	"github.com/gin-gonic/gin"
)

func setupRouter(
	engine *gin.Engine,
	authH *transactioncontroller.AuthHandler,
	captureH *transactioncontroller.CaptureHandler,
	voidH *transactioncontroller.VoidHandler,
	refundH *transactioncontroller.RefundHandler,
	productSvc *product.Service,
	purchaseSvc *purchase.Service,
) {
	api := engine.Group("/api/v1")
	{
		api.POST("/auth", authH.Handle)
		api.POST("/capture", captureH.Handle)
		api.POST("/void", voidH.Handle)
		api.POST("/refund", refundH.Handle)

		productcontroller.RegisterRoutes(
			api.Group("/products", merchantauth.Middleware()),
			productSvc,
		)
		purchasecontroller.RegisterRoutes(
			api.Group("/purchase", merchantauth.Middleware()),
			purchaseSvc,
		)
	}

	engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	engine.GET("/health/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
