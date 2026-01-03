# Feature 003: Inter-Service Communication

**Status:** In Progress

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
- [ ] Internal update endpoints в API service (`POST /internal/updates/orders`, `/internal/updates/disputes`)
- [ ] HTTP client в Ingest (`apiclient.Client` interface + `HTTPClient`)
- [ ] HTTPSyncProcessor що використовує apiclient
- [ ] WEBHOOK_MODE: `kafka` / `http`
- [ ] Integration tests
- [ ] k6 benchmark: Kafka vs HTTP

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

**Subtask 4:** Observability (optional)
- [ ] Health checks для обох сервісів
- [ ] Correlation IDs across services
- [ ] Basic metrics (latency, error rates)

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

### 2026-01-03: Feature created

Виділено з Feature 002. Оригінальний план передбачав gRPC одразу, але вирішено йти поступово:
1. HTTP - швидко отримати працюючу межу між сервісами
2. HTTP + Protobuf - практика з proto, ізольований бенчмарк серіалізації
3. gRPC - повна імплементація з усіма перевагами (streaming, multiplexing, etc.)
