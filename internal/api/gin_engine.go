package api

import (
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/metrics"

	"github.com/gin-gonic/gin"
)

func NewGinEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(
		logger.CorrelationMiddleware(), // Extract/generate correlation ID first
		metrics.GinMiddleware(),
		logger.GinBodyLogger(),
		gin.Recovery(),
	)
	return engine
}
