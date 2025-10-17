# Monitoring and Observability

This guide covers setting up monitoring, health checks, and observability for CLS Backend in production environments.

## Health Checks

### Built-in Health Endpoints

CLS Backend provides several health check endpoints for monitoring service health.

#### Service Health Check

```bash
# Basic health check
curl http://localhost:8080/health
```

**Healthy Response:**
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "pubsub": "healthy"
  },
  "timestamp": "2025-10-17T10:00:00Z"
}
```

**Unhealthy Response (503 Service Unavailable):**
```json
{
  "status": "unhealthy",
  "components": {
    "database": "unhealthy",
    "pubsub": "healthy"
  },
  "timestamp": "2025-10-17T10:00:00Z"
}
```

#### Service Information

```bash
# Get service version and build info
curl http://localhost:8080/api/v1/info
```

**Response:**
```json
{
  "service": "cls-backend",
  "version": "v1.0.0",
  "git_commit": "a1b2c3d",
  "build_time": "2025-10-17T10:00:00Z",
  "api_version": "v1",
  "environment": "production"
}
```

### Kubernetes Health Checks

#### Liveness Probe

```yaml
# In deployment.yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

#### Readiness Probe

```yaml
# In deployment.yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

## Metrics Collection

### Prometheus Metrics

CLS Backend exposes Prometheus metrics on port 8081.

#### Available Metrics

```bash
# Get all metrics
curl http://localhost:8080:8081/metrics
```

**Key Metrics:**
```prometheus
# HTTP Metrics
cls_backend_http_requests_total{method="GET",path="/clusters",status="200"} 1234
cls_backend_http_request_duration_seconds{method="GET",path="/clusters"} 0.045

# Database Metrics
cls_backend_database_connections_active 15
cls_backend_database_connections_idle 5
cls_backend_database_query_duration_seconds{operation="select"} 0.002

# Pub/Sub Metrics
cls_backend_pubsub_messages_published_total{topic="cluster-events"} 456
cls_backend_pubsub_publish_duration_seconds{topic="cluster-events"} 0.001

# Application Metrics
cls_backend_clusters_total 42
cls_backend_reconciliation_duration_seconds 0.123
```

#### Metrics Service

```yaml
# deploy/kubernetes/metrics-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: cls-backend-metrics
  namespace: cls-system
  labels:
    app: cls-backend
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8081"
    prometheus.io/path: "/metrics"
spec:
  ports:
  - name: metrics
    port: 8081
    targetPort: 8081
  selector:
    app: cls-backend
```

### Prometheus Configuration

#### ServiceMonitor (Prometheus Operator)

```yaml
# deploy/kubernetes/servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: cls-backend
  namespace: cls-system
  labels:
    app: cls-backend
spec:
  selector:
    matchLabels:
      app: cls-backend
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

#### Manual Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
- job_name: 'cls-backend'
  static_configs:
  - targets: ['cls-backend-metrics:8081']
  scrape_interval: 30s
  metrics_path: /metrics
```

## Logging

### Structured Logging Configuration

CLS Backend uses structured JSON logging. Configure via environment variables:

```yaml
# In ConfigMap
LOG_LEVEL: "info"           # debug, info, warn, error
LOG_FORMAT: "json"          # json, text
```

### Log Aggregation

#### Fluentd Configuration

```yaml
# fluentd-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluentd-config
data:
  fluent.conf: |
    <source>
      @type tail
      path /var/log/containers/cls-backend-*.log
      pos_file /var/log/fluentd-cls-backend.log.pos
      tag kubernetes.cls-backend
      format json
    </source>

    <match kubernetes.cls-backend>
      @type elasticsearch
      host elasticsearch.logging.svc.cluster.local
      port 9200
      index_name cls-backend
    </match>
```

#### Pod Annotations

```yaml
# In deployment.yaml
metadata:
  annotations:
    fluentd.kubernetes.io/log-format: json
    fluentd.kubernetes.io/parser-key: log
```

### Log Analysis Queries

#### Common Log Queries

```bash
# Get error logs from last hour
kubectl logs -l app=cls-backend --since=1h | grep '"level":"error"'

# Get database connection errors
kubectl logs -l app=cls-backend | grep '"component":"database"' | grep '"level":"error"'

# Get API request logs
kubectl logs -l app=cls-backend | grep '"component":"api"' | jq 'select(.method != null)'
```

## Alerting Rules

### Prometheus Alerting Rules

```yaml
# cls-backend-alerts.yaml
groups:
- name: cls-backend.rules
  rules:
  # Service Health Alerts
  - alert: CLSBackendDown
    expr: up{job="cls-backend"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "CLS Backend is down"
      description: "CLS Backend has been down for more than 1 minute"

  - alert: CLSBackendHighErrorRate
    expr: rate(cls_backend_http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate in CLS Backend"
      description: "Error rate is {{ $value }} errors per second"

  # Database Alerts
  - alert: DatabaseConnectionPoolExhausted
    expr: cls_backend_database_connections_active / cls_backend_database_connections_max > 0.9
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Database connection pool nearly exhausted"
      description: "Database connection pool usage is {{ $value | humanizePercentage }}"

  - alert: DatabaseSlowQueries
    expr: cls_backend_database_query_duration_seconds > 1.0
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Slow database queries detected"
      description: "Average query duration is {{ $value }}s"

  # Application Alerts
  - alert: HighReconciliationLatency
    expr: cls_backend_reconciliation_duration_seconds > 30
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High reconciliation latency"
      description: "Reconciliation taking {{ $value }}s to complete"

  - alert: TooManyFailedClusters
    expr: cls_backend_clusters_total{phase="Failed"} > 5
    for: 10m
    labels:
      severity: critical
    annotations:
      summary: "Too many failed clusters"
      description: "{{ $value }} clusters are in Failed state"
```

### Alertmanager Configuration

```yaml
# alertmanager.yml
route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'cls-backend-alerts'

receivers:
- name: 'cls-backend-alerts'
  slack_configs:
  - api_url: 'YOUR_SLACK_WEBHOOK_URL'
    channel: '#cls-alerts'
    title: 'CLS Backend Alert'
    text: '{{ range .Alerts }}{{ .Annotations.summary }}: {{ .Annotations.description }}{{ end }}'
```

## Grafana Dashboards

### CLS Backend Dashboard

```json
{
  "dashboard": {
    "title": "CLS Backend Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(cls_backend_http_requests_total[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(cls_backend_http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(cls_backend_http_requests_total{status=~\"5..\"}[5m])",
            "legendFormat": "5xx errors"
          }
        ]
      },
      {
        "title": "Database Connections",
        "type": "graph",
        "targets": [
          {
            "expr": "cls_backend_database_connections_active",
            "legendFormat": "Active"
          },
          {
            "expr": "cls_backend_database_connections_idle",
            "legendFormat": "Idle"
          }
        ]
      },
      {
        "title": "Cluster Status Distribution",
        "type": "pie",
        "targets": [
          {
            "expr": "cls_backend_clusters_total",
            "legendFormat": "{{phase}}"
          }
        ]
      }
    ]
  }
}
```

## Application-Level Monitoring

### Custom Health Checks

#### Database Health Check Script

```bash
#!/bin/bash
# database-health-check.sh

DATABASE_URL="$1"
THRESHOLD_MS=1000

if [ -z "$DATABASE_URL" ]; then
  echo "Usage: $0 <database_url>"
  exit 1
fi

# Measure query time
start_time=$(date +%s%3N)
result=$(psql "$DATABASE_URL" -c "SELECT 1" -t 2>/dev/null)
end_time=$(date +%s%3N)

duration=$((end_time - start_time))

if [ $? -eq 0 ] && [ "$result" = " 1" ]; then
  if [ $duration -lt $THRESHOLD_MS ]; then
    echo "Database healthy (${duration}ms)"
    exit 0
  else
    echo "Database slow (${duration}ms > ${THRESHOLD_MS}ms)"
    exit 1
  fi
else
  echo "Database connection failed"
  exit 1
fi
```

#### Pub/Sub Health Check

```bash
#!/bin/bash
# pubsub-health-check.sh

PROJECT_ID="$1"
TOPIC="cluster-events"

if [ -z "$PROJECT_ID" ]; then
  echo "Usage: $0 <project_id>"
  exit 1
fi

# Check topic exists
if gcloud pubsub topics describe "$TOPIC" --project="$PROJECT_ID" >/dev/null 2>&1; then
  echo "Pub/Sub topic healthy"
  exit 0
else
  echo "Pub/Sub topic check failed"
  exit 1
fi
```

### Application Performance Monitoring

#### Simple APM Script

```bash
#!/bin/bash
# apm-check.sh

BASE_URL="http://localhost:8080"
USER_EMAIL="monitoring@company.com"

# Measure API response times
measure_endpoint() {
  local endpoint="$1"
  local start_time=$(date +%s%3N)

  local response=$(curl -s -w "%{http_code}" -o /dev/null \
    -H "X-User-Email: $USER_EMAIL" \
    "$BASE_URL$endpoint")

  local end_time=$(date +%s%3N)
  local duration=$((end_time - start_time))

  echo "$endpoint: ${duration}ms (HTTP $response)"
}

echo "API Performance Check:"
measure_endpoint "/health"
measure_endpoint "/api/v1/info"
measure_endpoint "/api/v1/clusters"
```

## Troubleshooting Monitoring Issues

### Common Problems

#### 1. Metrics Not Appearing

**Check metrics endpoint:**
```bash
curl http://localhost:8080:8081/metrics
```

**Verify Prometheus configuration:**
```bash
# Check if target is being scraped
curl http://prometheus:9090/api/v1/targets
```

#### 2. Health Checks Failing

**Check service logs:**
```bash
kubectl logs -l app=cls-backend --tail=100
```

**Test health endpoint manually:**
```bash
kubectl port-forward service/cls-backend 8080:80
curl http://localhost:8080/health
```

#### 3. No Log Output

**Check log configuration:**
```bash
kubectl get configmap cls-backend-config -o yaml
```

**Verify log aggregation:**
```bash
kubectl logs -l app=fluentd | grep cls-backend
```

### Monitoring Best Practices

#### 1. Alert Configuration
- Set appropriate thresholds based on baseline performance
- Use different severity levels (info, warning, critical)
- Include actionable information in alert descriptions
- Test alert rules regularly

#### 2. Dashboard Design
- Group related metrics together
- Use consistent time ranges across panels
- Include both overview and detailed views
- Add annotations for deployments and incidents

#### 3. Log Management
- Use structured logging consistently
- Include correlation IDs for request tracing
- Set appropriate log retention policies
- Monitor log volume and costs

#### 4. Performance Monitoring
- Monitor key business metrics (cluster count, success rate)
- Track resource utilization (CPU, memory, connections)
- Set up synthetic monitoring for critical paths
- Regular performance testing and benchmarking

## Related Documentation

- **[Kubernetes Deployment](kubernetes.md)** - Complete deployment setup
- **[Configuration](configuration.md)** - Environment variables and settings
- **[Troubleshooting](../user-guide/troubleshooting.md)** - Common issues and solutions
- **[Architecture](../developer-guide/architecture.md)** - System design overview