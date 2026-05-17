package silvergate

import (
	"TestTaskJustPay/services/silvergate/internal/transaction/transactioncontroller"

	"github.com/gin-gonic/gin"
)

func setupRouter(engine *gin.Engine, authH *transactioncontroller.AuthHandler, captureH *transactioncontroller.CaptureHandler, voidH *transactioncontroller.VoidHandler, refundH *transactioncontroller.RefundHandler) {
	api := engine.Group("/api/v1")
	{
		api.POST("/auth", authH.Handle)
		api.POST("/capture", captureH.Handle)
		api.POST("/void", voidH.Handle)
		api.POST("/refund", refundH.Handle)
	}

	engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	engine.GET("/health/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
