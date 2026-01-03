# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working Mode: Learning-First

This is a **learning sandbox** for practicing highload/distributed systems concepts. The owner is a backend developer who has read DDIA and wants hands-on practice.

**How to work with me:**
1. **Plan first** - Always create a detailed plan before implementing. Explain the "why" behind architectural decisions.
2. **Implement** - Write the code, but expect that tests will be run manually by the user.
3. **Don't auto-fix test failures** - When tests fail, explain what went wrong and why, but let the user fix it themselves to learn.
4. **Explain trade-offs** - When implementing highload patterns, explain alternatives and why this approach was chosen.

**Language:**
- Conversation with user: Ukrainian is fine
- Code, comments, commits, PRs, documentation: **English only**

## Current Focus

**Active feature:** [docs/features/002-ingest-service-extraction/](docs/features/002-ingest-service-extraction/)

Full roadmap: [docs/roadmap.md](docs/roadmap.md)

## Feature Workflow

**On session start:**
1. Read this file and the active feature's `README.md`
2. Check which subtask is current (first unchecked in Tasks list)
3. Check if plan exists for that subtask (look for `plan-subtask-N.md` link)
4. If no plan - start planning phase
5. If plan exists - continue implementation from checkboxes

**Planning phase:**
1. Discuss approach with user, explain trade-offs and alternatives
2. When user approves the plan - save it to `docs/features/{feature-folder}/plan-subtask-N.md`
3. Add plan link to README.md
4. Then proceed to implementation

**During implementation:**
1. Follow the approved plan step by step
2. User runs tests manually and fixes issues themselves
3. Update checkboxes in feature file as tasks complete

**When feature is complete:**
1. Prompt the user: "Фіча завершена! Хочеш позначити її як done і перейти до наступної?"
2. If yes:
   - Mark feature status as Done in feature file
   - Update roadmap.md table
   - Update "Active feature" link in this file to next feature
   - Create next feature file from roadmap.md details

## Project Overview

This is a **Distributed Payment Manager** written in Go - a financial transaction management system that handles payment order lifecycle, dispute/chargeback management, and event sourcing. The system integrates with external payment providers (Silvergate) and uses PostgreSQL with time-series partitioning for high-performance event storage.

**Architecture:** The system consists of two microservices:
- **API Service** (`cmd/api`) - Core business logic, database owner, Kafka consumers, manual operations
- **Ingest Service** (`cmd/ingest`) - Lightweight HTTP → Kafka gateway for webhook ingestion

**Deployment modes:**
- **Sync mode** (dev): API service only, webhooks processed directly
- **Kafka mode** (prod): Both services, webhooks routed through Kafka

## Commands

### Development
```bash
make run-dev              # Sync mode: Start infrastructure + run API service locally (default)
make run-kafka            # Kafka mode: Start infrastructure + run both API + Ingest services (requires goreman)
make run-api              # Run API service only (standalone)
make run-ingest           # Run Ingest service only (standalone)
make start_containers     # Start only PostgreSQL, OpenSearch, Kafka, and Wiremock
make stop_containers      # Stop all containers
```

### Testing
```bash
make test                 # Run all unit tests with race detection
make integration-test     # Run integration tests (starts containers automatically)
make lint                 # Run golangci-lint
```

### Database
```bash
make migrate name=<name>  # Create new migration: make migrate name=add_user_table
make seed-db             # Seed database with test data
make clean-db            # Truncate all tables
make print-db-size       # Show current database size
```

### Other
```bash
make generate            # Run go generate ./... (generates mocks)
make benchmark           # Run k6 load tests
make build-pg-image      # Build custom PostgreSQL 17 image with pg_partman
```

### Running Tests
```bash
# Run single test
go test -v -run TestName ./path/to/package

# Run integration tests for specific package
go test -tags=integration -v ./internal/repo/dispute_eventsink

# Run with race detection
go test -race ./internal/domain/order
```

## Architecture

### Service-Based Monorepo Architecture

The codebase follows a service-based monorepo structure with clean separation between microservices:

```
cmd/
├── api/                    # API Service entry point
│   └── main.go
└── ingest/                 # Ingest Service entry point
    └── main.go

internal/
├── api/                    # API Service (business operations, database owner)
│   ├── service.go          # Bootstrap and dependency injection
│   ├── router.go           # API routes (no webhooks)
│   ├── gin_engine.go       # HTTP server configuration
│   ├── migration.go        # Database migration runner
│   ├── migrations/         # Embedded SQL migrations (Goose format)
│   ├── workers.go          # Kafka consumer management
│   ├── handlers/           # HTTP handlers (service only, no processor)
│   │   ├── order.go        # GET, POST /orders/* operations
│   │   ├── dispute.go      # Dispute operations
│   │   └── chargeback.go   # Chargeback operations
│   └── consumers/          # Kafka consumer handlers
│       ├── order.go        # Processes order webhooks from Kafka
│       └── dispute.go      # Processes dispute webhooks from Kafka
│
├── ingest/                 # Ingest Service (lightweight HTTP → Kafka gateway)
│   ├── service.go          # Bootstrap (no database, no business logic)
│   ├── router.go           # Webhook routes only
│   └── handlers/           # Webhook handlers (processor only, no service)
│       ├── order.go        # POST /webhooks/payments/orders
│       └── chargeback.go   # POST /webhooks/payments/chargebacks
│
└── shared/                 # Shared code across services
    ├── domain/             # Core business logic (framework-agnostic)
    │   ├── order/          # Order aggregate, service, repository interface
    │   ├── dispute/        # Dispute aggregate, service, repository interface
    │   └── gateway/        # Payment provider abstraction (port)
    ├── repo/               # Data access implementations
    │   ├── order/          # PostgreSQL order repository
    │   ├── dispute/        # PostgreSQL dispute repository
    │   ├── order_eventsink/    # Order event persistence
    │   └── dispute_eventsink/  # Dispute event persistence (partitioned)
    ├── external/           # Third-party integrations
    │   ├── kafka/          # Kafka publishers and consumers
    │   ├── silvergate/     # Payment gateway client
    │   └── opensearch/     # Event indexing
    ├── webhook/            # Webhook processing
    │   ├── processor.go    # Processor interface
    │   ├── sync.go         # Sync processor (for sync mode)
    │   └── async.go        # Async processor (Kafka publisher)
    ├── messaging/          # Kafka consumer infrastructure
    └── testinfra/          # Shared test utilities

pkg/                        # Shared utilities
├── logger/                 # Zerolog wrapper
├── pointers/               # Pointer helpers
└── postgres/               # PostgreSQL utilities
```

### Key Architectural Patterns

**Domain-Driven Design**: Three bounded contexts (order, dispute, gateway) with clear aggregate roots and value objects.

**Repository Pattern**: All data access is abstracted behind interfaces defined in `internal/shared/domain/*/repo.go`, implemented in `internal/shared/repo/`.

**Transaction Support**: Repositories support `InTransaction(func(Repo) error)` pattern for atomic multi-step operations:
```go
orderRepo.InTransaction(func(txRepo order.OrderRepo) error {
    // Multiple operations within single transaction
})
```

**Event Sourcing**: All state changes create immutable events stored in `*_events` tables. Events are:
- Idempotent (keyed by `provider_event_id`)
- Persisted in PostgreSQL for authority
- Indexed in OpenSearch for analytics
- Time-series partitioned for performance

**Interface Segregation**: Domain layer defines interfaces (`OrderRepo`, `EventSink`, `Provider`), infrastructure implements them. This allows easy mocking with `mockgen`.

### Design Philosophy: Scalability as Configuration

This project follows a **"complex opt-in"** principle:
- Core business logic remains infrastructure-agnostic
- Highload patterns (partitioning, Kafka, sharding) are implemented as swappable adapters
- The system should be deployable in "simple mode" (single Postgres, no message queue) without code changes
- Interfaces like `EventSink`, `Provider`, `Consumer` enable this flexibility

When implementing new features, always ask: "Can this be configured to use a simpler alternative?"

### State Machines

**Order Status Flow**:
```
StatusCreated → StatusUpdated → StatusSuccess
                              → StatusFailed
```
- Orders can be placed `on_hold` with reasons: `manual_review`, `risk`
- Orders in final status (success/failed) cannot transition
- Orders on hold cannot be captured

**Dispute Status Flow**:
```
DisputeOpen → DisputeUnderReview → DisputeSubmitted → DisputeWon
                                                     → DisputeLost
                                                     → DisputeClosed
                                  → DisputeCanceled
```

### Database Schema

**Time-Series Partitioning**: The `dispute_events` table uses PostgreSQL `pg_partman` for daily partitioning by `created_at`. This optimization reduces I/O from ~200MB to ~30MB for multi-day queries (see "Postgres Time Series Partitioning Notes.md" for detailed analysis).

**Key Tables**:
- `orders`: Order entities with status tracking and hold flags
- `order_events`: Event log for orders (webhook_received, hold_set, etc.)
- `disputes`: Dispute entities with evidence deadlines and status
- `dispute_events`: Event log for disputes (time-series partitioned)
- `evidence`: Evidence submissions linked to disputes

**Indexing Strategy**:
- Composite B-tree on `(kind, created_at)` for event queries
- Composite index on `(status, reason, id)` for dispute queries
- Unique constraint on `(order_id, provider_event_id)` for idempotency

### Testing Architecture

**Unit Tests**: Use `pgxmock/v4` for repository tests, `uber/mock` for service tests. Mocks are generated via `go generate ./...` from interface definitions.

**Integration Tests**: Tagged with `//go:build integration`. They:
- Start PostgreSQL/OpenSearch containers automatically
- Load test fixtures from `testdata/` directories
- Test end-to-end flows with real database
- Run via `make integration-test`

## Important Patterns & Conventions

### Error Handling
Domain-specific errors are defined in `internal/shared/domain/*/errors.go` and map to HTTP status codes in handlers. Always return typed errors from services.

### Query Building
Use Squirrel query builder for type-safe SQL. Pagination uses cursor-based approach with `id` and `created_at` for stable ordering.

### Configuration
Environment variables are parsed in `config/config.go` using `caarlos0/env/v11`. Required vars will cause startup failure. See `.env.example` for all options.

### Logging
Use structured logging via `pkg/logger/`. Log contexts include `order_id`, `dispute_id`, `correlation_id` for traceability.

### Code Generation
Run `make generate` after modifying interfaces to regenerate mocks. Mock files are named `mock_*.go` and excluded from git via patterns.

## External Dependencies

### Silvergate (Payment Provider)
- **Purpose**: Payment capture and dispute representment
- **Configuration**: `SILVERGATE_BASE_URL`, `SILVERGATE_SUBMIT_REPRESENTMENT_PATH`, `SILVERGATE_CAPTURE_PATH`
- **Mocking**: Wiremock stubs in `integration-test/mappings/` for local dev

### OpenSearch
- **Purpose**: Event indexing for analytics and audit trails
- **Indices**: `events-disputes`, `events-orders`
- **Local Access**: OpenSearch Dashboards at http://localhost:5601
- **Note**: Optional - app continues if OpenSearch is unavailable

### PostgreSQL
- **Version**: PostgreSQL 17 with `pg_partman` extension
- **Custom Image**: Built via `make build-pg-image` (see `PG.Dockerfile`)
- **Migrations**: Managed with `pressly/goose/v3` in `internal/app/migrations/`

## Development Workflow

1. **Setup**: Copy `.env.example` to `.env` and adjust values
2. **Start**: `make run-dev` (starts containers + runs app)
3. **Test**: `make test` for unit tests, `make integration-test` for integration
4. **Migrate**: `make migrate name=description` to create new migration
5. **Lint**: `make lint` before committing

## Migration Management

Migrations use Goose SQL format. Create new migrations with:
```bash
make migrate name=add_feature_table
```

This creates two files in `internal/app/migrations/`:
- `<timestamp>_add_feature_table.sql` - up migration
- (rollback in same file with `-- +goose Down` comment)

Migrations run automatically on app startup. For time-series tables, follow the pattern in `20250830122235_ts_partition_dispute_events.sql`.

## Performance Considerations

- **Cursor-based pagination**: Always use `cursor` + `limit` for event queries
- **Time-series partitioning**: Enabled for `dispute_events` table
- **Connection pooling**: Controlled via `PG_POOL_MAX` (default: 2 for dev)
- **Index strategy**: Composite indices on `(kind, created_at)` for event tables
- **Idempotency**: Use `provider_event_id` to prevent duplicate event processing

## Common Gotchas

- **Transaction boundaries**: Always use `InTransaction` for multi-step operations that must be atomic
- **Status validation**: Domain services validate state transitions - don't bypass them
- **Event idempotency**: The system relies on unique `provider_event_id` - never reuse them
- **Integration tests**: They modify the database - use `make clean-db` between manual runs
- **OpenSearch errors**: Non-critical - app logs errors but continues operating
