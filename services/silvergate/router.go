package silvergate

import (
	"TestTaskJustPay/services/silvergate/handlers"

	"github.com/gin-gonic/gin"
)

func setupRouter(engine *gin.Engine, authH *handlers.AuthHandler, captureH *handlers.CaptureHandler) {
	api := engine.Group("/api/v1")
	{
		api.POST("/auth", authH.Handle)
		api.POST("/capture", captureH.Handle)
	}

	engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	engine.GET("/health/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
