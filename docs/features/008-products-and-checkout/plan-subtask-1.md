# Plan: Restructure Paymanager to Package-by-Feature (Variant B)

## Goal

Reorganize `services/paymanager` from package-by-layer to package-by-feature. Each domain
(`payment`, `dispute`, `order`) becomes a self-contained directory with sub-packages for
HTTP handlers, Kafka consumers, and PostgreSQL implementations. The domain root package
contains entities, interfaces, and business logic — everything for one domain in one place.

## Current State

```
handlers/               ← all HTTP handlers mixed together
consumers/              ← all Kafka consumers mixed together
domain/order/           ← entity, service, interfaces
domain/payment/         ← entity, service, interfaces
domain/dispute/         ← entity, service, interfaces
domain/gateway/         ← shared Provider interface + all request/response types
domain/events/          ← shared event store interface + types
repo/order/             ← PG implementation
repo/payment/           ← PG implementation
repo/dispute/           ← PG implementation
repo/order_eventsink/   ← PG event sink implementation
repo/dispute_eventsink/ ← PG event sink implementation
repo/events/            ← PG event store implementation
dto/                    ← Kafka message types
```

## Architectural Decisions

| Question | Decision | Why |
|----------|----------|-----|
| Package layout | Package-by-feature (Variant B) | All domain code in one place; easier navigation; aligns with SOLID |
| Provider interface | Per-domain ISP — each domain defines its own minimal `Provider` interface | `dispute` doesn't need to know about `CapturePayment`; `payment` doesn't need `SubmitRepresentment`; silvergate client satisfies all implicitly |
| `gateway` package fate | Keep as shared DTOs only — remove `Provider` interface, rename `port.go` → `types.go` | Request/response types (AuthRequest, CaptureRequest, etc.) are provider-specific structs shared across domains; duplicating them adds no value |
| `events` package | Move from `domain/events/` to top-level `events/` — stay shared | Truly cross-cutting infrastructure used by all domains inside the same transaction |
| `order` domain | Restructure to Variant B but keep all logic intact | Legacy cleanup (remove CapturePayment endpoint) is Subtask 2 scope — no business changes here |

## Target Package Structure

```
services/paymanager/
├── app.go                      ← updated imports and DI wiring
├── router.go                   ← updated to use new handler subpackages
├── internal_router.go          ← updated
├── workers.go                  ← updated to use new consumer subpackages
│
├── payment/                    ← package payment
│   ├── entity.go               ← from domain/payment/entity.go
│   ├── errors.go               ← from domain/payment/errors.go
│   ├── repo.go                 ← from domain/payment/repo.go (PaymentRepo interface)
│   ├── service.go              ← from domain/payment/service.go
│   │                              + Provider interface defined here (auth/capture/void/refund)
│   ├── handler/                ← package handler
│   │   └── handler.go          ← from handlers/payment.go
│   ├── consumer/               ← package consumer
│   │   └── consumer.go         ← from consumers/payment.go
│   │                              + CaptureWebhook DTO moved here from dto/
│   └── postgres/               ← package postgres
│       └── repo.go             ← from repo/payment/pg_payment_repo.go
│
├── dispute/                    ← package dispute
│   ├── chargeback_entity.go    ← from domain/dispute/chargeback_entity.go
│   ├── dispute_entity.go       ← from domain/dispute/dispute_entity.go
│   ├── evidence_entity.go      ← from domain/dispute/evidence_entity.go
│   ├── errors.go               ← from domain/dispute/errors.go
│   ├── repo.go                 ← from domain/dispute/repo.go (DisputeRepo interface)
│   ├── event_sink.go           ← from domain/dispute/event_sink.go (DisputeEvents interface)
│   ├── service.go              ← from domain/dispute/service.go
│   │                              + Provider interface defined here (SubmitRepresentment only)
│   ├── handler/                ← package handler
│   │   └── handler.go          ← merged handlers/dispute.go + handlers/chargeback.go
│   ├── consumer/               ← package consumer
│   │   └── consumer.go         ← from consumers/dispute.go
│   │                              + ChargebackWebhook DTO moved here from dto/
│   └── postgres/               ← package postgres
│       ├── repo.go             ← from repo/dispute/pg_dispute_repo.go
│       └── event_sink.go       ← from repo/dispute_eventsink/pg_dispute_event_sink.go
│
├── order/                      ← package order (LEGACY — layout only, logic unchanged)
│   ├── order_entity.go         ← from domain/order/order_entity.go
│   ├── payment_entity.go       ← from domain/order/payment_entity.go
│   ├── errors.go               ← from domain/order/errors.go
│   ├── repo.go                 ← from domain/order/repo.go
│   ├── event_sink.go           ← from domain/order/event_sink.go
│   ├── service.go              ← from domain/order/service.go
│   │                              + Provider interface (CapturePayment — removed in Subtask 2)
│   ├── handler/                ← package handler
│   │   └── handler.go          ← from handlers/order.go
│   ├── consumer/               ← package consumer
│   │   └── consumer.go         ← from consumers/order.go
│   │                              + OrderUpdate DTO moved here from dto/
│   └── postgres/               ← package postgres
│       ├── repo.go             ← from repo/order/pg_order_repo.go
│       └── event_sink.go       ← from repo/order_eventsink/pg_order_event_sink.go
│
├── gateway/                    ← shared provider DTOs only (no interface)
│   └── types.go                ← AuthRequest, CaptureRequest, VoidRequest, RefundRequest,
│                                  RepresentmentRequest, and their result types
│                                  (extracted from domain/gateway/port.go)
│
└── events/                     ← shared event store (moved from domain/events/)
    ├── event.go
    └── errors.go
```

## Provider Interface Split (ISP example)

```go
// payment/service.go
type Provider interface {
    AuthorizePayment(ctx context.Context, req gateway.AuthRequest) (gateway.AuthResult, error)
    CapturePayment(ctx context.Context, req gateway.CaptureRequest) (gateway.CaptureResult, error)
    VoidPayment(ctx context.Context, req gateway.VoidRequest) (gateway.VoidResult, error)
    RefundPayment(ctx context.Context, req gateway.RefundRequest) (gateway.RefundResult, error)
}

// dispute/service.go
type Provider interface {
    SubmitRepresentment(ctx context.Context, req gateway.RepresentmentRequest) (gateway.RepresentmentResult, error)
}

// order/service.go
type Provider interface {
    CapturePayment(ctx context.Context, req gateway.CaptureRequest) (gateway.CaptureResult, error)
}
// ^ will be removed in Subtask 2 when order.CapturePayment endpoint is deleted
```

`external/silvergate/client.go` satisfies all three interfaces implicitly — no changes needed.

## Files to Delete After Restructuring

- `domain/` (entire directory)
- `handlers/` (entire directory)
- `consumers/` (entire directory)
- `repo/` (entire directory)
- `dto/` (entire directory)

## Implementation Order

1. **Move `events/`** — copy `domain/events/` to `events/`, update module path in all files that import it
2. **Refactor `gateway/`** — remove `Provider` interface from `domain/gateway/port.go`, rename to `gateway/types.go`
3. **Restructure `payment/`** — move entity/errors/repo/service files, create handler/ consumer/ postgres/ subpackages, define local `Provider` interface in service.go
4. **Restructure `dispute/`** — same; merge `handlers/chargeback.go` into `dispute/handler/handler.go`
5. **Restructure `order/`** — same; keep all logic exactly as-is
6. **Update `app.go`** — update all import paths; use import aliases for subpackages (e.g. `paymenthandler "…/payment/handler"`)
7. **Update `router.go` and `internal_router.go`** — use new handler package types
8. **Update `workers.go`** — use new consumer package types
9. **Delete old directories** — `domain/`, `handlers/`, `consumers/`, `repo/`, `dto/`
10. **Run `make test`** — verify nothing broke
