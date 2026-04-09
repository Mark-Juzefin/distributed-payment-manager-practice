package main

import (
	analytics "TestTaskJustPay/services/analytics"
	"TestTaskJustPay/services/analytics/config"
	"log"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	analytics.Run(cfg)
}
