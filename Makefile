-include env/common.env
export

MIGRATION_DIR=services/paymanager/migrations

.PHONY: run run-dev run-kafka run-http run-inbox run-minimal run-paymanager run-ingest run-silvergate start_containers start_containers_minimal stop_containers stop_containers_remove stop_containers_minimal lint test integration-test e2e-test generate migrate seed-db print-db-size clean-db benchmark build-pg-image test-webhook loadtest loadtest-steady patroni-status

run:
	docker compose --profile prod up --build

# Kafka mode: default dev mode (API + Ingest via Kafka)
run-kafka: start_containers
	@echo "Running in KAFKA mode (API + Ingest services)"
	go run github.com/mattn/goreman@latest start

# Alias for intuitive naming
run-dev: run-kafka

# HTTP mode: both services via HTTP sync
run-http: start_containers
	@echo "Running in HTTP mode (API + Ingest services)"
	go run github.com/mattn/goreman@latest -f Procfile.http start

# Inbox mode: Ingest writes to PostgreSQL inbox table
run-inbox: start_containers
	@echo "Running in INBOX mode (API + Ingest with PostgreSQL inbox)"
	go run github.com/mattn/goreman@latest -f Procfile.inbox start

# Light mode: HTTP mode with minimal infra (just PostgreSQL, no Kafka/OpenSearch/Patroni)
run-minimal: start_containers_minimal
	@echo "Running in MINIMAL mode (HTTP, standalone PostgreSQL)"
	go run github.com/mattn/goreman@latest -f Procfile.http start

start_containers_minimal:
	docker compose -f docker-compose.minimal.yaml up -d --wait

stop_containers_minimal:
	docker compose -f docker-compose.minimal.yaml down --remove-orphans

# Standalone targets
run-paymanager: start_containers
	go run ./services/paymanager/cmd

run-ingest:
	go run ./services/ingest/cmd

run-silvergate: start_containers
	set -a && source env/common.env && source env/endpoints.host.env && source env/silvergate.env && set +a && PORT=$${SILVERGATE_PORT} go run ./services/silvergate/cmd

start_containers:
	docker-compose --profile infra up --build -d --wait

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
	./services/paymanager/repo/dispute_eventsink \
	./services/paymanager/repo/order_eventsink \
	./services/paymanager/repo/events \
	./services/ingest/repo/inbox

integration-test:
	go clean -testcache && go test -tags=integration -v  $(INTEGRATION_DIRS)

.PHONY: integration-test-name
integration-test-name:
ifndef name
	$(error "Usage: make integration-test-name name=testname")
endif
	go clean -testcache && go test -run $(name)  -tags=integration -v  $(INTEGRATION_DIRS)

# E2E tests: Docker-based, real service containers
e2e-test:
	go clean -testcache && go test -tags=integration -v -timeout 5m ./e2e/...


generate:
	cd services/paymanager && go generate ./...
	cd services/ingest && go generate ./...

migrate:
ifndef name
	$(error "Usage: make migrate name=your_migration_name")
endif
	go tool goose -dir=$(MIGRATION_DIR) create $(name) sql

db-primary:
	PGPASSWORD=secret psql -h localhost -p 5440 -U postgres -d payments

db-replica:
	PGPASSWORD=secret psql -h localhost -p 5441 -U postgres -d payments

# Patroni cluster status
patroni-status:
	docker exec patroni1 patronictl -c /etc/patroni/patroni.yml list

seed-db:
	psql -d "$(PG_URL)" -f ./benchmark/generate_dispute_events.sql

print-db-size:
	psql -d "$(PG_URL)" -c 'SELECT pg_size_pretty(pg_database_size(current_database()));'

clean-db:
	psql -d "$(PG_URL)" -c  'TRUNCATE TABLE events, dispute_events, disputes, order_events, orders, evidence CASCADE'

# Load test: generate realistic data via webhook flow
loadtest:
	go run ./loadtest -target http://localhost:3001 -vus 10 -duration 30s

# Steady load: continuous traffic until Ctrl+C (for observing Grafana dashboards)
loadtest-steady:
	go run ./loadtest -target http://localhost:3001 -vus 5

# Test webhook: sends via Ingest service (full flow)
test-webhook:
	@./scripts/send-test-webhook.sh created
