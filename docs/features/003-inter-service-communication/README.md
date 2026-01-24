# Feature 003: Inter-Service Communication

**Status:** Paused

## Overview

Реалізація sync mode комунікації між Ingest та API сервісами. Поступовий перехід від HTTP до gRPC з проміжним етапом HTTP + Protobuf.

**Мотивація:**
- Чітка межа між сервісами (HTTP/gRPC замість прямого виклику)
- Підготовка до gRPC через проміжні кроки
- Можливість бенчмаркінгу різних підходів (Kafka vs HTTP vs gRPC)
- Практика з Protocol Buffers та gRPC

**Архітектура:**

```
Kafka mode (async, production):
  Webhook → Ingest → Kafka → API consumer → domain logic

HTTP sync mode:
  Webhook → Ingest → HTTP → API endpoint → domain logic

gRPC sync mode (target):
  Webhook → Ingest → gRPC → API server → domain logic
```

## Subtasks

**Subtask 1:** HTTP Sync Mode — [plan-subtask-1.md](plan-subtask-1.md) | [notes.md](notes.md)
- [x] Internal update endpoints в API service (`POST /internal/updates/orders`, `/internal/updates/disputes`)
- [x] HTTP client в Ingest (`apiclient.Client` interface + `HTTPClient`)
- [x] HTTPSyncProcessor що використовує apiclient
- [x] WEBHOOK_MODE: `kafka` / `http`
- [x] Unit tests for new components
- [ ] k6 benchmark: Kafka vs HTTP (moved E2E tests to Subtask 5)

**Subtask 2:** HTTP + Protobuf
- [ ] Proto definitions для webhook payloads
- [ ] Protobuf serialization замість JSON
- [ ] Benchmark: JSON vs Protobuf over HTTP

**Subtask 3:** gRPC
- [ ] gRPC service definition
- [ ] gRPC server в API service
- [ ] gRPC client в Ingest (`GRPCSyncProcessor`)
- [ ] WEBHOOK_MODE: `kafka` / `http` / `grpc`
- [ ] Benchmark: HTTP vs gRPC

**Subtask 4:** ~~Observability~~ → Moved to [Feature 004](../004-observability/)

**Subtask 5:** E2E Test Refactoring — [plan-subtask-5.md](plan-subtask-5.md)
- [ ] Process-based test infrastructure (запуск сервісів як окремих процесів)
- [ ] E2E tests for Kafka mode
- [ ] E2E tests for HTTP mode
- [ ] Видалити дублювання setupTestServer (~75 рядків)
- [ ] Makefile targets: e2e-test, e2e-test-kafka, e2e-test-http

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
