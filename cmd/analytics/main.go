package main

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/analytics"
	"log"
)

func main() {
	cfg, err := config.NewAnalyticsConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	analytics.Run(cfg)
}
