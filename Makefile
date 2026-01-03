-include .env
export

MIGRATION_DIR=internal/api/migrations

.PHONY: run run-dev run-sync run-kafka run-api run-ingest start_containers stop_containers stop_containers_remove lint test integration-test generate migrate seed-db print-db-size clean-db benchmark build-pg-image

run:
	docker compose --profile prod up --build

# Default: sync mode (simple for dev, API service only)
run-dev: start_containers
	@echo "Running in SYNC mode (API service only)"
	WEBHOOK_MODE=sync go run ./cmd/api

# Kafka mode: both services (requires goreman or two terminals)
run-kafka: start_containers
	@which goreman > /dev/null || (echo "Install goreman: go install github.com/mattn/goreman@latest" && exit 1)
	@echo "Running in KAFKA mode (API + Ingest services)"
	goreman start

# Standalone targets
run-api: start_containers
	go run ./cmd/api

run-ingest:
	go run ./cmd/ingest

# Backward compatibility
run-sync: run-dev

start_containers:
	docker-compose --profile infra up --build -d

stop_containers:
	docker compose --profile infra --profile prod down --remove-orphans

stop_containers_remove:
	docker compose --profile infra down -v

build-pg-image:
	docker build -f PG.Dockerfile -t pg17-partman:local .

lint:
	golangci-lint run

test:
	go test -race ./...

INTEGRATION_DIRS = \
	./integration-test/... \
	./internal/api/repo/dispute_eventsink \
	./internal/api/repo/order_eventsink

integration-test:
	go clean -testcache && go test -tags=integration -v  $(INTEGRATION_DIRS)

.PHONY: integration-test-name
integration-test-name:
ifndef name
	$(error "Usage: make integration-test-name name=testname")
endif
	go clean -testcache && go test -run $(name)  -tags=integration -v  $(INTEGRATION_DIRS)


generate:
	go generate ./...

migrate:
ifndef name
	$(error "Usage: make migrate name=your_migration_name")
endif
	go tool goose -dir=$(MIGRATION_DIR) create $(name) sql

seed-db:
	psql -d "$(PG_URL)" -f ./benchmark/generate_dispute_events.sql

print-db-size:
	psql -d "$(PG_URL)" -c 'SELECT pg_size_pretty(pg_database_size(current_database()));'

clean-db:
	psql -d "$(PG_URL)" -c  'TRUNCATE TABLE dispute_events, disputes, order_events, orders, evidence CASCADE'

benchmark:
	k6 run -e BASE_URL=http://localhost:3000 -e LIMIT=1000 -e VUS=8 -e DURATION=30s benchmark/disputes_bench.js