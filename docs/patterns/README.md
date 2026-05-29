# Architecture & Patterns Reference

A **portable, code-grounded catalogue** of the patterns used in this repository
(a distributed payment manager written in Go). It is meant to be pointed at from
*other* projects: instead of parsing the whole codebase, an agent reads the one
relevant file here and gets the pattern, the rationale, and a link to the
canonical source.

> **For agents:** When a user says *"implement transactions like in
> `<this-repo>/docs/patterns/transactions.md`"*, read that single file. It
> contains the contract, a minimal real snippet, the trade-off, and `path:line`
> pointers to the source of truth. Open the source files only if you need more
> than the snippet shows.

## How to use this from another project

1. Pick the pattern file below that matches the task.
2. Read it — it is self-contained (signatures + a short real snippet + why).
3. If you need the full implementation, follow the `path:line` references.
   All paths are **relative to this repository's root**, not to this folder.
4. **Adapt, don't copy verbatim.** These patterns assume the stack listed below;
   match the target project's idioms.

## Pattern files

| File | Covers |
|------|--------|
| [transactions.md](transactions.md) | DB transactions & consistency: `Transactor`/`Executor` abstraction, repo-factory pattern, explicit isolation levels, `SELECT FOR UPDATE`, compare-and-swap, reserve/release, idempotency keys, side-effects outside the tx |
| [ddd-structure.md](ddd-structure.md) | Code layout & DDD: Go-workspace monorepo, per-domain `internal/` packages, guarded aggregates / state machines, domain-error sentinels, ports & adapters, "complex opt-in", manual DI |
| [messaging.md](messaging.md) | Async messaging: composable consumer middleware (retry/DLQ/metrics), manual-commit at-least-once consumer, outbox → CDC → analytics, inbox + polling worker, swappable webhook processor |
| [infra-testing.md](infra-testing.md) | Plumbing & tests: env-based config, pgx pool + Squirrel, embedded Goose migrations, hand-rolled fakes vs `mockgen`, testcontainers integration suites, migration-test rule |

## The one idea that ties it together

**Scalability as configuration ("complex opt-in").** Core business logic is
infrastructure-agnostic; the heavy patterns (Kafka, CDC, sharding, inbox) are
swappable adapters behind interfaces (`Executor`, `Acquirer`, `Processor`,
`MessageHandler`). The same domain code runs in "simple mode" (single Postgres,
no broker) or "complex mode" without changes. When you lift a pattern from here,
keep that seam: depend on the interface, make the heavy implementation opt-in.

## Stack assumed by the snippets

- **Language:** Go (module path `TestTaskJustPay`, Go workspaces / `go.work`)
- **Postgres:** `jackc/pgx/v5` (+ `pgxpool`), `Masterminds/squirrel` query builder (`$N` placeholders)
- **HTTP:** `gin-gonic/gin`
- **Kafka:** `segmentio/kafka-go`
- **Migrations:** `pressly/goose/v3` (embedded `embed.FS`)
- **Config:** `caarlos0/env/v11`
- **CDC:** `jackc/pglogrepl` (Postgres logical replication)
- **Tests:** stdlib `testing`, `testcontainers-go`, `uber/mock` (mockgen) where mocks are generated

## Architecture at a glance

Four services in an isolated Go workspace (each its own module, cannot import
another — enforced by the compiler):

- **paymanager** — core domain, DB owner, Kafka consumers, manual ops
- **silvergate** — mock payment provider (PSP): products, auth/capture/refund/void, `/purchase` composition
- **ingest** — HTTP → Kafka/inbox webhook gateway
- **cdc** — Postgres WAL → Kafka change data capture
- **analytics** — Kafka → OpenSearch projection

Shared code lives in `pkg/` (`postgres`, `kafka`, `messaging`, `migrations`,
`logger`, `metrics`, `health`, `testinfra`). The richest, most current patterns
live in **`services/silvergate/internal/`** — prefer it as a model over older
code.
