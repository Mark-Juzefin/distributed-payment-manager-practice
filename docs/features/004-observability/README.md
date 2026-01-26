# Feature 004: Observability

**Status:** In Progress

## Overview

Повноцінна observability для мікросервісної архітектури: метрики, дашборди, трейсинг, профілювання.

**Мотивація:**
- Без метрик неможливо приймати рішення про оптимізації (JSON vs Protobuf, HTTP vs gRPC)
- Практика з Prometheus/Grafana — індустріальний стандарт
- Distributed tracing критичний для дебагу мікросервісів
- SLO-based thinking — основа надійності

**Перенесено з Feature 003:**
- Health checks для сервісів
- Correlation IDs across services
- Basic metrics (latency, error rates)

## Subtasks

**Subtask 1:** HTTP Metrics — [plan-subtask-1.md](plan-subtask-1.md)
- [x] Prometheus instrumentation (prometheus/client_golang)
- [x] `/metrics` endpoint для обох сервісів
- [x] HTTP handler latency histogram (p50/p95/p99)
- [x] Request counter by endpoint and status

**Subtask 2:** Kafka Metrics — [plan-subtask-2.md](plan-subtask-2.md)
- [ ] Kafka consumer lag metric
- [x] Kafka message processing duration histogram

**Subtask 3:** Health Checks — [plan-subtask-3.md](plan-subtask-3.md)
- [x] `/health/live` — liveness (process alive)
- [x] `/health/ready` — readiness (dependencies OK: DB, Kafka)
- [x] Health check handlers

**Subtask 4:** Correlation IDs
- [ ] Generate/propagate X-Correlation-ID header
- [ ] Include in all log entries
- [ ] Pass through Kafka messages

**Subtask 5:** Grafana Dashboards
- [ ] Docker compose з Prometheus + Grafana
- [ ] Service health dashboard (RPS, latency, errors)
- [ ] Kafka dashboard (lag, throughput)

**Subtask 6:** Distributed Tracing (optional)
- [ ] OpenTelemetry SDK integration
- [ ] Jaeger для візуалізації
- [ ] Trace propagation через HTTP та Kafka

**Subtask 7:** Profiling (optional)
- [ ] pprof endpoints (`/debug/pprof/`)
- [ ] Basic profiling documentation

**Subtask 8:** Dev Infrastructure Refactoring
- [x] Simplify environment variable management
- [x] Unify run modes (sync/kafka/http)
- [x] Improve local development experience

---

## Architecture Decision Records

### ADR-1: Prometheus over custom metrics

**Decision:** Використовувати Prometheus client library.

**Rationale:**
- Індустріальний стандарт
- Pull-based model простіший (не треба push gateway)
- Багато готових інтеграцій (Grafana, alerting)
- Go client library добре підтримується

---

## Notes

### 2026-01-25: Dev Infrastructure Refactoring

Реорганізація env файлів для локальної розробки:

```
env/
├── common.env           # Ports, Kafka topics
├── endpoints.host.env   # localhost URLs (PG, Kafka, OpenSearch, Silvergate)
├── endpoints.docker.env # docker URLs (для docker-compose)
├── api.env              # API-specific config
└── ingest.env           # Ingest-specific config
```

Зміни:
- Прибрано дублювання портів та URLs між файлами
- `run-dev` тепер аліас до `run-http` (default dev mode)
- `run-kafka` для тестування Kafka-specific логіки
- Goreman запускається через `go run github.com/mattn/goreman@latest` (без manual install)
- DLQ topics створюються автоматично в kafka-init
- Видалено застарілі `.env.*.example` файли

### 2026-01-23: Feature created

Виділено з Feature 003 (Inter-Service Communication) для фокусу на observability як prerequisite для бенчмарків HTTP vs Protobuf vs gRPC.
