package main

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/app"
	"log"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	app.Run(cfg)
}
