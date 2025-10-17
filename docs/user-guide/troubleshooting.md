# Troubleshooting Guide

This guide helps you diagnose and resolve common issues when using CLS Backend.

## Service Health Issues

### Service Won't Start

#### Database Connection Failed

**Symptoms:**
```
ERROR Failed to connect to database: connection refused
```

**Solutions:**
1. **Check DATABASE_URL format**:
   ```bash
   export DATABASE_URL="postgres://user:password@host:5432/database"
   ```

2. **Verify PostgreSQL is running**:
   ```bash
   # Local PostgreSQL
   pg_isready -h localhost -p 5432

   # Remote PostgreSQL
   pg_isready -h your-db-host -p 5432
   ```

3. **Test connection manually**:
   ```bash
   psql "$DATABASE_URL"
   ```

4. **Check database permissions**:
   ```sql
   -- Connect as admin and verify user permissions
   \du  -- List users and roles
   ```

#### Pub/Sub Initialization Failed

**Symptoms:**
```
ERROR Failed to initialize Pub/Sub client: authentication failed
```

**Solutions:**
1. **Verify GOOGLE_CLOUD_PROJECT**:
   ```bash
   echo $GOOGLE_CLOUD_PROJECT
   ```

2. **Check GCP authentication**:
   ```bash
   gcloud auth list
   gcloud auth application-default login
   ```

3. **Verify Pub/Sub API is enabled**:
   ```bash
   gcloud services list --enabled | grep pubsub
   ```

4. **Test Pub/Sub access**:
   ```bash
   gcloud pubsub topics list
   ```

#### Port Already in Use

**Symptoms:**
```
ERROR Failed to start server: bind: address already in use
```

**Solutions:**
1. **Check what's using the port**:
   ```bash
   lsof -i :8080
   ```

2. **Use a different port**:
   ```bash
   export PORT=8081
   ./bin/backend-api
   ```

3. **Kill the conflicting process**:
   ```bash
   kill $(lsof -t -i:8080)
   ```

## API Request Issues

### Authentication Errors

#### 401 Unauthorized

**Symptoms:**
```json
{
  "error": "Authentication required"
}
```

**Solutions:**
1. **Add X-User-Email header**:
   ```bash
   curl -H "X-User-Email: user@example.com" \
     http://localhost:8080/api/v1/clusters
   ```

2. **For development, disable auth**:
   ```bash
   export DISABLE_AUTH=true
   ```

#### 403 Forbidden

**Symptoms:**
```json
{
  "error": "Access denied"
}
```

**Solutions:**
1. **Check user permissions** (in production with external auth)
2. **Verify user email format**:
   ```bash
   # Use valid email format
   -H "X-User-Email: user@domain.com"
   ```

### Resource Not Found

#### 404 Not Found

**Symptoms:**
```json
{
  "error": "Cluster not found"
}
```

**Solutions:**
1. **Verify cluster ID**:
   ```bash
   # List clusters to get correct IDs
   curl -H "X-User-Email: user@example.com" \
     http://localhost:8080/api/v1/clusters
   ```

2. **Check user context** (clusters are filtered by user):
   ```bash
   # Use same X-User-Email for create and retrieve
   ```

3. **Verify endpoint URL**:
   ```bash
   # Correct: /api/v1/clusters/{id}
   # Incorrect: /api/v1/organizations/{org}/clusters/{id}
   ```

### Request Format Issues

#### 400 Bad Request

**Symptoms:**
```json
{
  "error": "Invalid JSON format"
}
```

**Solutions:**
1. **Check JSON syntax**:
   ```bash
   # Validate JSON before sending
   echo '{"name": "test"}' | jq .
   ```

2. **Verify Content-Type header**:
   ```bash
   curl -H "Content-Type: application/json" \
        -H "X-User-Email: user@example.com" \
        -d '{"name": "test"}' \
        http://localhost:8080/api/v1/clusters
   ```

3. **Check required fields**:
   ```json
   {
     "name": "required-field",
     "spec": {
       "platform": {
         "type": "gcp"
       }
     }
   }
   ```

#### 409 Conflict

**Symptoms:**
```json
{
  "error": "Cluster with name 'my-cluster' already exists"
}
```

**Solutions:**
1. **Use unique cluster names**:
   ```bash
   # Add timestamp or UUID
   CLUSTER_NAME="my-cluster-$(date +%s)"
   ```

2. **Delete existing cluster first**:
   ```bash
   curl -X DELETE \
     -H "X-User-Email: user@example.com" \
     http://localhost:8080/api/v1/clusters/{existing-id}
   ```

## Status and Monitoring Issues

### Cluster Stuck in Pending

**Symptoms:**
- Cluster phase remains "Pending" for extended time
- No controllers reporting status

**Solutions:**
1. **Check if controllers are running**:
   ```bash
   # If using Kubernetes
   kubectl get pods -l app=controller
   ```

2. **Verify Pub/Sub subscriptions**:
   ```bash
   gcloud pubsub subscriptions list
   ```

3. **Check cluster-events topic**:
   ```bash
   gcloud pubsub topics list | grep cluster-events
   ```

4. **Monitor reconciliation events**:
   ```bash
   # Check service logs for reconciliation activity
   kubectl logs -f deployment/cls-backend
   ```

### Status Not Updating

**Symptoms:**
- Cluster status shows stale information
- Controllers reporting but status unchanged

**Solutions:**
1. **Check controller status endpoint**:
   ```bash
   curl -H "X-User-Email: user@example.com" \
     http://localhost:8080/api/v1/clusters/{id}/status
   ```

2. **Verify generation matching**:
   - Controllers must report status for current cluster generation
   - Check controller logs for generation mismatches

3. **Force status recalculation**:
   ```bash
   # Update cluster to trigger status refresh
   curl -X PUT http://localhost:8080/api/v1/clusters/{id} \
     -H "Content-Type: application/json" \
     -H "X-User-Email: user@example.com" \
     -d '{"spec": {...}}'
   ```

### Reconciliation Issues

#### Clusters Not Being Reconciled

**Symptoms:**
- No reconciliation events in logs
- Clusters remain in stale states

**Solutions:**
1. **Check reconciliation scheduler**:
   ```bash
   # Look for scheduler startup in logs
   grep "reconciliation scheduler" /var/log/cls-backend.log
   ```

2. **Verify reconciliation configuration**:
   ```bash
   export RECONCILIATION_ENABLED=true
   export RECONCILIATION_CHECK_INTERVAL=1m
   ```

3. **Check database reconciliation schedule**:
   ```sql
   SELECT cluster_id, next_reconcile_at, enabled
   FROM reconciliation_schedule
   WHERE enabled = true;
   ```

## Performance Issues

### Slow API Responses

**Symptoms:**
- API requests taking > 5 seconds
- Timeouts on cluster operations

**Solutions:**
1. **Check database performance**:
   ```sql
   -- Check for slow queries
   SELECT query, mean_time, calls
   FROM pg_stat_statements
   ORDER BY mean_time DESC
   LIMIT 10;
   ```

2. **Monitor connection pool**:
   ```bash
   # Check metrics endpoint
   curl http://localhost:8080:8081/metrics | grep database
   ```

3. **Verify indexes**:
   ```sql
   -- Check for missing indexes
   SELECT schemaname, tablename, attname, n_distinct, correlation
   FROM pg_stats
   WHERE tablename IN ('clusters', 'controller_status');
   ```

### High Memory Usage

**Symptoms:**
- Service consuming excessive memory
- Out of memory errors

**Solutions:**
1. **Check database connection limits**:
   ```bash
   export DATABASE_MAX_OPEN_CONNS=25
   export DATABASE_MAX_IDLE_CONNS=5
   ```

2. **Monitor Go memory metrics**:
   ```bash
   curl http://localhost:8080:8081/metrics | grep go_memstats
   ```

3. **Restart service if memory leak suspected**:
   ```bash
   kubectl rollout restart deployment/cls-backend
   ```

## Development Issues

### Tests Failing

#### Unit Tests

**Symptoms:**
```
FAIL: TestClusterCreation
```

**Solutions:**
1. **Run tests with verbose output**:
   ```bash
   go test -v ./internal/...
   ```

2. **Check test database**:
   ```bash
   export DATABASE_URL="postgres://test:test@localhost:5432/cls_test"
   ```

3. **Clean test environment**:
   ```bash
   make clean
   make test-unit
   ```

#### Integration Tests

**Symptoms:**
- Tests pass locally but fail in CI
- Database connection issues in tests

**Solutions:**
1. **Verify test dependencies**:
   ```bash
   # Ensure PostgreSQL and Pub/Sub emulator are running
   docker-compose up -d postgres pubsub-emulator
   ```

2. **Check test isolation**:
   ```bash
   # Run tests in isolation
   go test -count=1 ./internal/api/
   ```

### Build Issues

#### Go Module Issues

**Symptoms:**
```
go: module not found
```

**Solutions:**
1. **Update dependencies**:
   ```bash
   go mod download
   go mod tidy
   ```

2. **Clear module cache**:
   ```bash
   go clean -modcache
   ```

#### Container Build Issues

**Symptoms:**
```
ERROR: failed to build image
```

**Solutions:**
1. **Check Docker/Podman installation**:
   ```bash
   docker version
   podman version
   ```

2. **Verify build context**:
   ```bash
   # Ensure you're in the correct directory
   ls Dockerfile go.mod
   ```

3. **Use correct platform**:
   ```bash
   podman build --platform linux/amd64 -t cls-backend .
   ```

## Getting Help

### Log Analysis

1. **Service logs**:
   ```bash
   # Local development
   tail -f /var/log/cls-backend.log

   # Kubernetes
   kubectl logs -f deployment/cls-backend -n cls-system
   ```

2. **Database logs**:
   ```bash
   # Check PostgreSQL logs for connection issues
   tail -f /var/log/postgresql/postgresql.log
   ```

3. **Enable debug logging**:
   ```bash
   export LOG_LEVEL=debug
   ```

### Health Endpoints

```bash
# Basic health check
curl http://localhost:8080/health

# Service information
curl http://localhost:8080/api/v1/info

# Metrics (if enabled)
curl http://localhost:8080:8081/metrics
```

### Community Support

- [GitHub Issues](https://github.com/your-org/cls-backend/issues)
- [GitHub Discussions](https://github.com/your-org/cls-backend/discussions)
- [API Reference](../reference/api.md)
- [Developer Guide](../developer-guide/)

When reporting issues, please include:
- Service version
- Error messages
- Configuration (sanitized)
- Steps to reproduce
- Environment details (OS, Go version, etc.)