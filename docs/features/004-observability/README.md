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

**Subtask 1:** Metrics Foundation — [plan-subtask-1.md](plan-subtask-1.md)
- [ ] Prometheus instrumentation (prometheus/client_golang)
- [ ] HTTP handler latency histogram (p50/p95/p99)
- [ ] Request counter by endpoint and status
- [ ] Kafka consumer lag metric
- [ ] `/metrics` endpoint для обох сервісів

**Subtask 2:** Health Checks
- [ ] `/health/live` — liveness (process alive)
- [ ] `/health/ready` — readiness (dependencies OK: DB, Kafka)
- [ ] Health check middleware

**Subtask 3:** Correlation IDs
- [ ] Generate/propagate X-Correlation-ID header
- [ ] Include in all log entries
- [ ] Pass through Kafka messages

**Subtask 4:** Grafana Dashboards
- [ ] Docker compose з Prometheus + Grafana
- [ ] Service health dashboard (RPS, latency, errors)
- [ ] Kafka dashboard (lag, throughput)

**Subtask 5:** Distributed Tracing (optional)
- [ ] OpenTelemetry SDK integration
- [ ] Jaeger для візуалізації
- [ ] Trace propagation через HTTP та Kafka

**Subtask 6:** Profiling (optional)
- [ ] pprof endpoints (`/debug/pprof/`)
- [ ] Basic profiling documentation

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

### 2026-01-23: Feature created

Виділено з Feature 003 (Inter-Service Communication) для фокусу на observability як prerequisite для бенчмарків HTTP vs Protobuf vs gRPC.
