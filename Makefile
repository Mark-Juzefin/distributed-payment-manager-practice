-include .env
export

MIGRATION_DIR=src/app/migration

.PHONY: run run_dev start_containers stop_containers migrate

run:
	docker compose --profile prod up --build

run_dev: start_containers
	go run ./cmd/app

start_containers:
	docker-compose --profile infra up -d

stop_containers:
	docker compose --profile infra --profile prod down --remove-orphans

migrate:
ifndef name
	$(error "Usage: make migrate name=your_migration_name")
endif
	go tool goose -dir=$(MIGRATION_DIR) create $(name) sql
