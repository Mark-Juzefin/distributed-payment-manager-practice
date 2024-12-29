package app

import (
	"TestTaskJustPay/src/api"
	"github.com/gin-gonic/gin"
)

type Server struct {
	engine    *gin.Engine
	apiRouter *api.Router
}

func (s *Server) Start() error {
	s.apiRouter.SetUp(s.engine)
	//todo: handle error
	return s.engine.Run()
}

func NewServer(engine *gin.Engine, apiRouter *api.Router) *Server {
	return &Server{
		engine:    engine,
		apiRouter: apiRouter,
	}
}
