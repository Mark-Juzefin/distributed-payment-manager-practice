# Plan: Grafana Dashboards

## Goal

Add Prometheus + Grafana infrastructure to Docker Compose with auto-provisioned dashboards for monitoring service health and Kafka performance.

## Current State

**Metrics already implemented:**
- `dpm_http_request_duration_seconds{handler, method, status_code}` — histogram
- `dpm_http_requests_total{handler, method, status_code}` — counter
- `dpm_kafka_message_processing_duration_seconds{topic, consumer_group, status}` — histogram
- `dpm_kafka_messages_processed_total{topic, consumer_group, status}` — counter
- Go runtime metrics (goroutines, memory, GC)
- Process metrics (CPU, file descriptors)

**Endpoints:**
- API service: `localhost:3000/metrics`
- Ingest service: `localhost:3001/metrics`

## Architecture Decisions

| Question | Decision | Why |
|----------|----------|-----|
| Directory structure | `monitoring/` in project root | Clean separation from app code |
| Docker profile | `monitoring` (separate from `infra`) | Lightweight dev mode without metrics stack |
| Grafana port | 3100 | Avoid conflict with API on 3000 |
| Dashboard provisioning | File-based provisioning | Dashboards as code, version controlled |
| Prometheus scrape interval | 15s | Balance between granularity and overhead |

## File Structure

```
monitoring/
├── prometheus.yml                    # Scrape configuration
└── grafana/
    ├── provisioning/
    │   ├── datasources/
    │   │   └── prometheus.yml        # Auto-configure Prometheus datasource
    │   └── dashboards/
    │       └── default.yml           # Dashboard provider config
    └── dashboards/
        ├── service-health.json       # HTTP metrics dashboard
        └── kafka.json                # Kafka metrics dashboard
```

## Implementation

### 1. Docker Compose Services

Add to `docker-compose.yaml`:

```yaml
prometheus:
  image: prom/prometheus:v2.51.0
  container_name: prometheus
  profiles:
    - monitoring
  networks:
    - backend
  ports:
    - "9090:9090"
  volumes:
    - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    - prometheus-data:/prometheus
  command:
    - '--config.file=/etc/prometheus/prometheus.yml'
    - '--storage.tsdb.path=/prometheus'
    - '--web.enable-lifecycle'

grafana:
  image: grafana/grafana:10.4.0
  container_name: grafana
  profiles:
    - monitoring
  networks:
    - backend
  ports:
    - "3100:3000"
  environment:
    - GF_SECURITY_ADMIN_USER=admin
    - GF_SECURITY_ADMIN_PASSWORD=admin
    - GF_USERS_ALLOW_SIGN_UP=false
  volumes:
    - ./monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
    - ./monitoring/grafana/dashboards:/var/lib/grafana/dashboards:ro
    - grafana-data:/var/lib/grafana
  depends_on:
    - prometheus
```

Add volumes:
```yaml
volumes:
  prometheus-data:
  grafana-data:
```

### 2. Prometheus Configuration

`monitoring/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'api-service'
    static_configs:
      - targets: ['host.docker.internal:3000']
    metrics_path: /metrics

  - job_name: 'ingest-service'
    static_configs:
      - targets: ['host.docker.internal:3001']
    metrics_path: /metrics
```

Note: `host.docker.internal` allows Prometheus (in Docker) to scrape services running on host machine.

### 3. Grafana Datasource Provisioning

`monitoring/grafana/provisioning/datasources/prometheus.yml`:

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
```

### 4. Grafana Dashboard Provider

`monitoring/grafana/provisioning/dashboards/default.yml`:

```yaml
apiVersion: 1

providers:
  - name: 'default'
    orgId: 1
    folder: 'DPM'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    options:
      path: /var/lib/grafana/dashboards
```

### 5. Service Health Dashboard

`monitoring/grafana/dashboards/service-health.json`:

**Panels:**

| Row | Panel | Query | Visualization |
|-----|-------|-------|---------------|
| 1 | RPS (all) | `sum(rate(dpm_http_requests_total[1m]))` | Stat |
| 1 | Error Rate % | `sum(rate(dpm_http_requests_total{status_code=~"5.."}[1m])) / sum(rate(dpm_http_requests_total[1m])) * 100` | Stat (red threshold) |
| 1 | Avg Latency | `sum(rate(dpm_http_request_duration_seconds_sum[1m])) / sum(rate(dpm_http_request_duration_seconds_count[1m]))` | Stat |
| 2 | RPS by Handler | `sum by (handler) (rate(dpm_http_requests_total[1m]))` | Time series |
| 2 | Latency p50/p95/p99 | `histogram_quantile(0.95, sum by (le) (rate(dpm_http_request_duration_seconds_bucket[1m])))` | Time series |
| 3 | Requests by Status | `sum by (status_code) (rate(dpm_http_requests_total[1m]))` | Pie chart |
| 3 | Error Rate Over Time | `sum(rate(dpm_http_requests_total{status_code=~"5.."}[1m]))` | Time series |
| 4 | Go Routines | `go_goroutines` | Time series |
| 4 | Memory Usage | `process_resident_memory_bytes / 1024 / 1024` | Time series (MB) |

**Variables:**
- `job`: selector for api-service / ingest-service

### 6. Kafka Dashboard

`monitoring/grafana/dashboards/kafka.json`:

**Panels:**

| Row | Panel | Query | Visualization |
|-----|-------|-------|---------------|
| 1 | Throughput | `sum(rate(dpm_kafka_messages_processed_total[1m]))` | Stat |
| 1 | Success Rate % | `sum(rate(dpm_kafka_messages_processed_total{status="success"}[1m])) / sum(rate(dpm_kafka_messages_processed_total[1m])) * 100` | Stat |
| 1 | Avg Processing Time | `sum(rate(dpm_kafka_message_processing_duration_seconds_sum[1m])) / sum(rate(dpm_kafka_message_processing_duration_seconds_count[1m]))` | Stat |
| 2 | Messages by Topic | `sum by (topic) (rate(dpm_kafka_messages_processed_total[1m]))` | Time series |
| 2 | Processing Duration p95 | `histogram_quantile(0.95, sum by (le, topic) (rate(dpm_kafka_message_processing_duration_seconds_bucket[1m])))` | Time series |
| 3 | Success vs Failure | `sum by (status) (rate(dpm_kafka_messages_processed_total[1m]))` | Pie chart |
| 3 | Failures Over Time | `sum(rate(dpm_kafka_messages_processed_total{status="error"}[1m]))` | Time series |

**Variables:**
- `topic`: selector for webhooks.orders / webhooks.disputes
- `consumer_group`: selector

## Files to Modify

| File | Changes |
|------|---------|
| `docker-compose.yaml` | Add prometheus and grafana services, volumes |
| `monitoring/prometheus.yml` | New file - scrape config |
| `monitoring/grafana/provisioning/datasources/prometheus.yml` | New file |
| `monitoring/grafana/provisioning/dashboards/default.yml` | New file |
| `monitoring/grafana/dashboards/service-health.json` | New file |
| `monitoring/grafana/dashboards/kafka.json` | New file |
| `Makefile` | Add `start-monitoring` target (optional) |

## Implementation Order

1. Create `monitoring/` directory structure
2. Create `monitoring/prometheus.yml`
3. Create Grafana provisioning files (datasources, dashboard provider)
4. Add Prometheus + Grafana to `docker-compose.yaml`
5. Test: `docker compose --profile monitoring up -d` and verify Prometheus targets
6. Create `service-health.json` dashboard
7. Create `kafka.json` dashboard
8. Test dashboards with running services
9. (Optional) Add Makefile target for convenience

## Testing

```bash
# Start monitoring stack
docker compose --profile monitoring up -d

# Verify Prometheus is scraping
curl http://localhost:9090/api/v1/targets

# Access Grafana
open http://localhost:3100
# Login: admin/admin

# Generate some traffic to see metrics
curl http://localhost:3000/orders
curl http://localhost:3000/health/ready
```

## Notes

- Dashboards are JSON files exported from Grafana UI — will create basic structure manually then refine in UI if needed
- `host.docker.internal` works on Docker Desktop (Mac/Windows); for Linux may need `--add-host` or `extra_hosts`
- Consumer lag metric is not implemented yet (Subtask 2) — Kafka dashboard will show only processing metrics for now
