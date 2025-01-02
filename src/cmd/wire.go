//go:build wireinject
// +build wireinject

package main

import (
	"TestTaskJustPay/src/api"
	"TestTaskJustPay/src/api/handlers"
	"TestTaskJustPay/src/app"
	"TestTaskJustPay/src/config"
	"TestTaskJustPay/src/pkg"
	order_repo "TestTaskJustPay/src/repo/order"
	"TestTaskJustPay/src/service/order"
	"github.com/google/wire"
)

func InitServer() (*app.Server, error) {
	wire.Build(
		order.NewOrderService,
		handlers.NewOrderHandler,
		api.NewRouter,
		app.NewServer,
		app.NewGinEngine,
		config.New,
		pkg.NewPgPool,
		order_repo.NewRepo,
		wire.Bind(new(order.Repo), new(*order_repo.Repo)))
	return &app.Server{}, nil
}
