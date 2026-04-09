package main

import (
	ingest "TestTaskJustPay/services/ingest"
	"TestTaskJustPay/services/ingest/config"
	"log"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	ingest.Run(cfg)
}
