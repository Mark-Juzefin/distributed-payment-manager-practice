package main

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/cdc"
	"log"
)

func main() {
	cfg, err := config.NewCDCConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	cdc.Run(cfg)
}
