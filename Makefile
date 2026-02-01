-include env/common.env
export

MIGRATION_DIR=internal/api/migrations

.PHONY: run run-dev run-kafka run-http run-api run-ingest start_containers start-monitoring stop_containers stop_containers_remove lint test integration-test generate migrate seed-db print-db-size clean-db benchmark build-pg-image test-webhook

run:
	docker compose --profile prod up --build

# HTTP mode: default dev mode (API + Ingest via HTTP)
run-http: start_containers
	@echo "Running in HTTP mode (API + Ingest services)"
	go run github.com/mattn/goreman@latest -f Procfile.http start

# Alias for intuitive naming
run-dev: run-http

# Kafka mode: both services via Kafka
run-kafka: start_containers
	@echo "Running in KAFKA mode (API + Ingest services)"
	go run github.com/mattn/goreman@latest start

# Standalone targets
run-api: start_containers
	go run ./cmd/api

run-ingest:
	go run ./cmd/ingest

start_containers:
	docker-compose --profile infra up --build -d

stop_containers:
	docker compose --profile infra --profile prod down --remove-orphans

stop_containers_remove:
	docker compose --profile infra down -v

start-monitoring:
	docker compose --profile monitoring up -d

stop-monitoring:
	docker compose --profile monitoring down -v


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

# Test webhook: sends via Ingest service (full flow)
test-webhook:
	@./scripts/send-test-webhook.sh created