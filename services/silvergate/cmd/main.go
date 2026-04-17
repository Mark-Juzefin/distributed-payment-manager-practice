package main

import (
	"log"

	silvergate "TestTaskJustPay/services/silvergate"
	"TestTaskJustPay/services/silvergate/config"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	app, err := silvergate.NewApp(cfg)
	if err != nil {
		log.Fatalf("failed to create app: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("app error: %v", err)
	}
}
