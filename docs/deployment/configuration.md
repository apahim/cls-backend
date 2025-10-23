# Configuration Guide

This document provides comprehensive configuration information for CLS Backend, including environment variables, security settings, and deployment-specific configurations.

## Configuration Overview

CLS Backend uses environment variables for configuration, following the [12-factor app](https://12factor.net/config) methodology. Configuration is organized into logical groups for easier management.

## Environment Variables

### Required Configuration

These variables must be set for the application to start:

#### Database Configuration

```bash
# PostgreSQL connection string
DATABASE_URL=postgres://username:password@host:5432/database?sslmode=require

# Connection pool settings (optional, with defaults)
DATABASE_MAX_OPEN_CONNS=25    # Maximum open connections
DATABASE_MAX_IDLE_CONNS=5     # Maximum idle connections
DATABASE_CONN_MAX_LIFETIME=1h # Connection lifetime
```

**Examples:**
```bash
# Local development
DATABASE_URL=postgres://cls_user:cls_password@localhost:5432/cls_backend?sslmode=disable

# Cloud SQL with SSL
DATABASE_URL=postgres://cls_user:password@10.0.0.1:5432/cls_backend?sslmode=require

# Cloud SQL with Unix socket
DATABASE_URL=postgres://cls_user:password@/cls_backend?host=/cloudsql/project:region:instance
```

#### Google Cloud Configuration

```bash
# GCP project ID for Pub/Sub and other services
GOOGLE_CLOUD_PROJECT=your-gcp-project-id

# Service account authentication (if not using workload identity)
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
```

### Server Configuration

```bash
# HTTP server port (default: 8080)
PORT=8080

# Server environment (development, staging, production)
ENVIRONMENT=production

# Graceful shutdown timeout (default: 30s)
SHUTDOWN_TIMEOUT=30s

# Request timeout (default: 30s)
REQUEST_TIMEOUT=30s
```

### Authentication Configuration

```bash
# Disable authentication (development only)
DISABLE_AUTH=false

# External authorization settings (future use)
AUTH_PROVIDER=external
AUTH_HEADER_USER_EMAIL=X-User-Email
AUTH_HEADER_USER_ROLES=X-User-Roles
```

**Authentication Modes:**

- **Development**: `DISABLE_AUTH=true` - No authentication required
- **Production**: `DISABLE_AUTH=false` - External authorization via API Gateway

### Pub/Sub Configuration (Simplified Fan-Out)

```bash
# Primary topic for all cluster events
PUBSUB_CLUSTER_EVENTS_TOPIC=cluster-events

# Publisher settings
PUBSUB_PUBLISH_TIMEOUT=10s
PUBSUB_PUBLISH_RETRY_ATTEMPTS=3

# Subscription settings (for testing/debugging only)
PUBSUB_RECEIVE_TIMEOUT=10s
PUBSUB_MAX_OUTSTANDING_MESSAGES=1000
```

**Fan-Out Architecture:**
- **Single Topic**: `cluster-events` for all cluster lifecycle events
- **Controller Subscriptions**: Controllers create their own subscriptions
- **Self-Filtering**: Controllers filter events based on platform and dependencies

### Reconciliation Configuration (Binary State System)

```bash
# Enable/disable reconciliation scheduler
RECONCILIATION_ENABLED=true

# How often to check for clusters needing reconciliation
RECONCILIATION_CHECK_INTERVAL=1m

# Maximum concurrent reconciliation operations
RECONCILIATION_MAX_CONCURRENT=50

# Default interval (used only for fallback)
RECONCILIATION_DEFAULT_INTERVAL=5m
```

**Binary State Intervals:**
- **Needs Attention**: 30 seconds (automatic, not configurable)
- **Stable**: 5 minutes (automatic, not configurable)

### Logging Configuration

```bash
# Log level (debug, info, warn, error)
LOG_LEVEL=info

# Log format (json, console)
LOG_FORMAT=json

# Enable request logging
LOG_REQUESTS=true

# Log database queries (debug only)
LOG_DATABASE_QUERIES=false
```

### Metrics Configuration

```bash
# Enable metrics collection
METRICS_ENABLED=true

# Metrics server port
METRICS_PORT=8081

# Metrics path
METRICS_PATH=/metrics

# Enable pprof debugging endpoints
PPROF_ENABLED=false
```

## Configuration Profiles

### Development Profile

```bash
# Development environment configuration
ENVIRONMENT=development
DISABLE_AUTH=true
LOG_LEVEL=debug
LOG_FORMAT=console
METRICS_ENABLED=true
PPROF_ENABLED=true

# Local database
DATABASE_URL=postgres://cls_user:cls_password@localhost:5432/cls_backend?sslmode=disable

# Local Pub/Sub emulator (optional)
PUBSUB_EMULATOR_HOST=localhost:8085
```

### Staging Profile

```bash
# Staging environment configuration
ENVIRONMENT=staging
DISABLE_AUTH=false
LOG_LEVEL=info
LOG_FORMAT=json
METRICS_ENABLED=true

# Staging database with SSL
DATABASE_URL=postgres://cls_user:password@staging-db:5432/cls_backend?sslmode=require

# Staging GCP project
GOOGLE_CLOUD_PROJECT=my-project-staging
```

### Production Profile

```bash
# Production environment configuration
ENVIRONMENT=production
DISABLE_AUTH=false
LOG_LEVEL=info
LOG_FORMAT=json
METRICS_ENABLED=true
PPROF_ENABLED=false

# Production database with connection pooling
DATABASE_URL=postgres://cls_user:password@prod-db:5432/cls_backend?sslmode=require
DATABASE_MAX_OPEN_CONNS=50
DATABASE_MAX_IDLE_CONNS=10

# Production GCP project
GOOGLE_CLOUD_PROJECT=my-project-production

# Production reconciliation settings
RECONCILIATION_MAX_CONCURRENT=100
```

## Kubernetes Configuration

### ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cls-backend-config
  namespace: cls-system
data:
  # Server settings
  PORT: "8080"
  ENVIRONMENT: "production"
  SHUTDOWN_TIMEOUT: "30s"
  REQUEST_TIMEOUT: "30s"

  # Authentication
  DISABLE_AUTH: "false"
  AUTH_PROVIDER: "external"
  AUTH_HEADER_USER_EMAIL: "X-User-Email"

  # Pub/Sub (simplified fan-out)
  PUBSUB_CLUSTER_EVENTS_TOPIC: "cluster-events"
  PUBSUB_PUBLISH_TIMEOUT: "10s"
  PUBSUB_PUBLISH_RETRY_ATTEMPTS: "3"

  # Database connection pool
  DATABASE_MAX_OPEN_CONNS: "25"
  DATABASE_MAX_IDLE_CONNS: "5"
  DATABASE_CONN_MAX_LIFETIME: "1h"

  # Reconciliation (binary state)
  RECONCILIATION_ENABLED: "true"
  RECONCILIATION_CHECK_INTERVAL: "1m"
  RECONCILIATION_MAX_CONCURRENT: "50"

  # Logging
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  LOG_REQUESTS: "true"
  LOG_DATABASE_QUERIES: "false"

  # Metrics
  METRICS_ENABLED: "true"
  METRICS_PORT: "8081"
  METRICS_PATH: "/metrics"
  PPROF_ENABLED: "false"
```

### Secret Configuration

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cls-backend-secrets
  namespace: cls-system
type: Opaque
stringData:
  # Database connection (with credentials)
  DATABASE_URL: "postgres://cls_user:secure-password@10.0.0.1:5432/cls_backend?sslmode=require"

  # GCP project ID
  GOOGLE_CLOUD_PROJECT: "my-production-project"
```

### Service Account Key Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cls-backend-gcp-key
  namespace: cls-system
type: Opaque
data:
  key.json: <base64-encoded-service-account-key>
```

## Security Configuration

### TLS Configuration

```bash
# Enable TLS for API server (if using HTTPS)
TLS_ENABLED=true
TLS_CERT_FILE=/etc/tls/tls.crt
TLS_KEY_FILE=/etc/tls/tls.key

# Database TLS settings (via DATABASE_URL)
DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require&sslcert=client.crt&sslkey=client.key&sslrootcert=ca.crt
```

### Authentication Headers

```bash
# Headers expected from external authorization system
AUTH_HEADER_USER_EMAIL=X-User-Email      # Required: user email
AUTH_HEADER_USER_DOMAIN=X-User-Domain    # Optional: user domain
AUTH_HEADER_USER_ROLES=X-User-Roles      # Optional: user roles
AUTH_HEADER_ORG_ID=X-Organization-ID     # Future: organization context
```

### CORS Configuration

```bash
# CORS settings (if serving browser clients)
CORS_ENABLED=true
CORS_ALLOWED_ORIGINS=https://my-frontend.com,https://admin.my-company.com
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Content-Type,Authorization,X-User-Email
CORS_ALLOW_CREDENTIALS=true
```

### External DNS Configuration

```bash
# Enable external-dns integration for automatic DNS record creation
EXTERNAL_DNS_ENABLED=false

# DNS hostname for the service (required if enabled)
EXTERNAL_DNS_HOSTNAME=cls-backend.example.com

# DNS zone (optional, used for validation)
EXTERNAL_DNS_ZONE=example.com

# TTL for DNS records in seconds (default: 300)
EXTERNAL_DNS_TTL=300
```

**External DNS Integration:**
- **Disabled by default** - Set `EXTERNAL_DNS_ENABLED=true` to enable
- **Requires hostname** - Must specify the full DNS hostname for the service
- **Automatic DNS records** - Creates A records pointing to LoadBalancer IP
- **Works with any DNS provider** - Supports Google Cloud DNS, AWS Route53, Cloudflare, etc.
- **Custom annotations** - Can add provider-specific annotations via Helm values

## Database Configuration

### Connection Pool Tuning

```bash
# Connection pool sizing
DATABASE_MAX_OPEN_CONNS=25      # Total connections
DATABASE_MAX_IDLE_CONNS=5       # Idle connections
DATABASE_CONN_MAX_LIFETIME=1h   # Connection lifetime

# Query timeouts
DATABASE_QUERY_TIMEOUT=30s      # Individual query timeout
DATABASE_MIGRATION_TIMEOUT=10m  # Migration timeout
```

### Performance Settings

```bash
# Database performance settings
DATABASE_LOG_SLOW_QUERIES=true
DATABASE_SLOW_QUERY_THRESHOLD=1s
DATABASE_ENABLE_PREPARED_STATEMENTS=true
```

### High Availability Settings

```bash
# Database failover configuration
DATABASE_PRIMARY_URL=postgres://user:pass@primary:5432/db
DATABASE_REPLICA_URL=postgres://user:pass@replica:5432/db
DATABASE_ENABLE_FAILOVER=true
DATABASE_FAILOVER_TIMEOUT=5s
```

## Monitoring Configuration

### Metrics Configuration

```bash
# Prometheus metrics
METRICS_ENABLED=true
METRICS_PORT=8081
METRICS_PATH=/metrics
METRICS_NAMESPACE=cls_backend

# Custom metrics
METRICS_COLLECT_DB_STATS=true
METRICS_COLLECT_PUBSUB_STATS=true
METRICS_COLLECT_RECONCILIATION_STATS=true
```

### Health Check Configuration

```bash
# Health check settings
HEALTH_CHECK_TIMEOUT=5s
HEALTH_CHECK_DATABASE=true
HEALTH_CHECK_PUBSUB=true

# Readiness vs liveness
READINESS_CHECK_DEPENDENCIES=true
LIVENESS_CHECK_BASIC_ONLY=true
```

### Tracing Configuration

```bash
# Distributed tracing (optional)
TRACING_ENABLED=false
TRACING_JAEGER_ENDPOINT=http://jaeger-collector:14268/api/traces
TRACING_SAMPLE_RATE=0.1
```

## Performance Tuning

### High-Traffic Configuration

```bash
# Server settings for high traffic
PORT=8080
REQUEST_TIMEOUT=30s
SHUTDOWN_TIMEOUT=60s
MAX_HEADER_SIZE=1MB

# Database settings for high load
DATABASE_MAX_OPEN_CONNS=100
DATABASE_MAX_IDLE_CONNS=25
DATABASE_CONN_MAX_LIFETIME=30m

# Reconciliation for high load
RECONCILIATION_MAX_CONCURRENT=200
RECONCILIATION_CHECK_INTERVAL=30s

# Pub/Sub for high throughput
PUBSUB_PUBLISH_TIMEOUT=5s
PUBSUB_MAX_OUTSTANDING_MESSAGES=5000
```

### Memory Optimization

```bash
# Memory usage optimization
DATABASE_MAX_OPEN_CONNS=10      # Reduce connections
DATABASE_MAX_IDLE_CONNS=2       # Reduce idle connections
LOG_LEVEL=warn                  # Reduce logging
METRICS_ENABLED=false           # Disable metrics if not needed
```

## Configuration Validation

### Startup Validation

The application validates configuration at startup:

```bash
# Required variables check
- DATABASE_URL must be valid PostgreSQL URL
- GOOGLE_CLOUD_PROJECT must be set
- PORT must be valid port number

# Value validation
- LOG_LEVEL must be: debug, info, warn, error
- LOG_FORMAT must be: json, console
- ENVIRONMENT must be: development, staging, production

# Dependency validation
- Database connection must succeed
- Pub/Sub authentication must succeed (if enabled)
```

### Configuration Testing

```bash
# Test configuration without starting server
./bin/backend-api --validate-config

# Test database connection
./bin/backend-api --test-database

# Test Pub/Sub connection
./bin/backend-api --test-pubsub
```

## Configuration Management

### Environment-Specific Configs

```bash
# Use different config files per environment
export ENV_FILE=/etc/cls-backend/production.env
export CONFIG_DIR=/etc/cls-backend/conf.d/

# Load configuration from multiple sources
source /etc/cls-backend/base.env
source /etc/cls-backend/${ENVIRONMENT}.env
```

### Secret Management

```bash
# Use external secret management
export SECRET_PROVIDER=vault
export VAULT_ADDR=https://vault.company.com
export VAULT_ROLE=cls-backend

# Or use Kubernetes secret projection
export DATABASE_URL_FILE=/var/secrets/database-url
export GCP_KEY_FILE=/var/secrets/gcp-key.json
```

### Configuration Hot Reload

Some configuration can be updated without restart:

```bash
# Hot-reloadable settings (via SIGHUP)
- LOG_LEVEL
- METRICS_ENABLED
- RECONCILIATION_CHECK_INTERVAL

# Requires restart
- DATABASE_URL
- PORT
- PUBSUB_CLUSTER_EVENTS_TOPIC
```

## Troubleshooting Configuration

### Common Issues

#### 1. Database Connection

```bash
# Test database URL format
psql "$DATABASE_URL" -c "SELECT version();"

# Check connection pool exhaustion
grep "connection pool" /var/log/cls-backend.log

# Monitor connections
SELECT count(*) FROM pg_stat_activity WHERE datname = 'cls_backend';
```

#### 2. Pub/Sub Authentication

```bash
# Test service account permissions
gcloud pubsub topics list --project=$GOOGLE_CLOUD_PROJECT

# Check Pub/Sub topic exists
gcloud pubsub topics describe cluster-events --project=$GOOGLE_CLOUD_PROJECT

# Verify credentials
gcloud auth list
```

#### 3. Configuration Conflicts

```bash
# Check environment variables
printenv | grep -E "(DATABASE|PUBSUB|RECONCILIATION|LOG)" | sort

# Validate configuration
./bin/backend-api --validate-config --verbose

# Check for conflicting settings
grep -E "(DISABLE_AUTH|ENVIRONMENT)" /var/log/cls-backend.log
```

### Configuration Debugging

```bash
# Enable configuration debugging
LOG_LEVEL=debug
LOG_CONFIG_ON_STARTUP=true

# Check effective configuration
curl http://localhost:8080/api/v1/debug/config

# Monitor configuration changes
tail -f /var/log/cls-backend.log | grep -i config
```

This configuration guide provides all the information needed to properly configure CLS Backend for any environment, from local development to high-scale production deployments.