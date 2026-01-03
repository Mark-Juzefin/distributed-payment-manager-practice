package api

import (
	"TestTaskJustPay/pkg/logger"

	"github.com/gin-gonic/gin"
)

func NewGinEngine(l *logger.Logger) *gin.Engine {
	engine := gin.New()
	engine.Use(l.GinBodyLogger(), gin.Recovery())
	return engine
}
