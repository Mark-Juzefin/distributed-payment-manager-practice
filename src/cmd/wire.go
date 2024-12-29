//go:build wireinject
// +build wireinject

package main

import (
	"TestTaskJustPay/src/api"
	"TestTaskJustPay/src/api/handlers"
	"TestTaskJustPay/src/app"
	"TestTaskJustPay/src/service"
	"github.com/google/wire"
)

func InitServer() *app.Server {
	wire.Build(
		service.NewOrderService,
		handlers.NewOrderHandler,
		api.NewRouter,
		app.NewServer,
		app.NewGinEngine)
	return &app.Server{}
}
