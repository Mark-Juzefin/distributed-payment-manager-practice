# Feature 003: Inter-Service Communication

**Status:** Done

## Overview

Реалізація sync mode комунікації між Ingest та API сервісами через HTTP.

**Результат:**
- Чітка межа між сервісами через HTTP endpoints
- Можливість вибору режиму: Kafka (async) або HTTP (sync)
- Internal API endpoints для service-to-service комунікації

**Архітектура:**

```
Kafka mode (async, production):
  Webhook → Ingest → Kafka → API consumer → domain logic

HTTP sync mode:
  Webhook → Ingest → HTTP → API endpoint → domain logic
```

## Subtasks

**Subtask 1:** HTTP Sync Mode — [plan-subtask-1.md](plan-subtask-1.md) | [notes.md](notes.md)
- [x] Internal update endpoints в API service (`POST /internal/updates/orders`, `/internal/updates/disputes`)
- [x] HTTP client в Ingest (`apiclient.Client` interface + `HTTPClient`)
- [x] HTTPSyncProcessor що використовує apiclient
- [x] WEBHOOK_MODE: `kafka` / `http`
- [x] Unit tests for new components
---

## Architecture Decision Records

### ADR-1: Progressive approach (HTTP → Protobuf → gRPC)

**Decision:** Реалізувати sync mode поступово, починаючи з HTTP.

**Rationale:**
- HTTP простіший для дебагу (curl, browser dev tools)
- Protobuf окремо від gRPC дозволяє ізолювати вплив серіалізації
- Кожен крок дає можливість для бенчмарку
- Менший ризик - якщо gRPC не потрібен, HTTP працює

### ADR-2: Internal endpoints

**Decision:** Використовувати `/internal/` prefix для service-to-service endpoints.

**Rationale:**
- Чітке розділення public vs internal API
- Можливість застосувати різні middleware (auth, rate limiting)
- Стандартна практика в мікросервісах

---

## Notes

## Implementation Log

### 2026-01-04: HTTP Sync Mode (Subtask 1 - partial)

**Completed:**
- Shared DTOs: `internal/shared/dto/order_update.go`, `dispute_update.go`
- API internal endpoints: `internal/api/handlers/updates/updates.go`, `internal_router.go`
- Ingest API client with retry: `internal/ingest/apiclient/` (client, errors, retry)
- HTTPSyncProcessor: `internal/ingest/webhook/http.go`
- Config updates for HTTP mode
- Makefile `run-http` target + `Procfile.http`
- Unit tests for client and processor
- Removed old unused SyncProcessor

**Note:** Package renamed from `handlers/internal/` to `handlers/updates/` — Go's internal package visibility rules blocked import from parent package.

**Remaining:**
- E2E integration test (HTTP mode)
- k6 benchmark: Kafka vs HTTP

---


### 2026-01-03: Feature created

Виділено з Feature 002. Оригінальний план передбачав gRPC одразу, але вирішено йти поступово:
1. HTTP - швидко отримати працюючу межу між сервісами
2. HTTP + Protobuf - практика з proto, ізольований бенчмарк серіалізації
3. gRPC - повна імплементація з усіма перевагами (streaming, multiplexing, etc.)
