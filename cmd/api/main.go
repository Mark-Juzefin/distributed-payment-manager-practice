package main

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/api"
	"log"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	api.Run(cfg)
}
