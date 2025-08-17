-include .env
export

MIGRATION_DIR=src/app/migration

.PHONY: run run-dev start_containers stop_containers lint test integration-test generate migrate

run:
	docker compose --profile prod up --build

run-dev: start_containers
	go run ./cmd/app

start_containers:
	docker-compose --profile infra up -d

stop_containers:
	docker compose --profile infra --profile prod down --remove-orphans

lint:
	golangci-lint run

test:
	go test -race ./...

integration-test: start_containers
	go clean -testcache && go test -tags=integration -v ./integration-test/...

generate:
	go generate ./...

migrate:
ifndef name
	$(error "Usage: make migrate name=your_migration_name")
endif
	go tool goose -dir=$(MIGRATION_DIR) create $(name) sql
