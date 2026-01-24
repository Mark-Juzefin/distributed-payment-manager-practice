package api

import (
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/metrics"

	"github.com/gin-gonic/gin"
)

func NewGinEngine(l *logger.Logger) *gin.Engine {
	engine := gin.New()
	engine.Use(metrics.GinMiddleware(), l.GinBodyLogger(), gin.Recovery())
	return engine
}
