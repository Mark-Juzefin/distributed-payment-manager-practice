package main

import (
	cdc "TestTaskJustPay/services/cdc"
	"TestTaskJustPay/services/cdc/config"
	"log"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	cdc.Run(cfg)
}
