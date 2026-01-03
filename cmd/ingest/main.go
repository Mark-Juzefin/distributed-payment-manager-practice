package main

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/ingest"
	"log"
)

func main() {
	cfg, err := config.NewIngestConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	ingest.Run(cfg)
}
