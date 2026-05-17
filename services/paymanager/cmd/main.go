package main

import (
	"TestTaskJustPay/services/paymanager"
	"TestTaskJustPay/services/paymanager/config"
	"log"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	paymanager.Run(cfg)
}
