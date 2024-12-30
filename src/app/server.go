package app

import (
	"TestTaskJustPay/src/api"
	"TestTaskJustPay/src/config"
	"embed"
	"fmt"
	"github.com/gin-gonic/gin"
)

//go:embed migration/*.sql
var MIGRATION_FS embed.FS

type Server struct {
	engine    *gin.Engine
	apiRouter *api.Router
	config    config.Config
}

func (s *Server) Start() error {
	s.apiRouter.SetUp(s.engine)

	fmt.Println("[1]", s.config.DB.String)

	err := applyMigrations(s.config.DB.String, MIGRATION_FS)
	if err != nil {
		return err
	}

	return s.engine.Run()
}

func NewServer(engine *gin.Engine, apiRouter *api.Router, config config.Config) *Server {
	return &Server{
		engine:    engine,
		apiRouter: apiRouter,
		config:    config,
	}
}
