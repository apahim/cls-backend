# Deployment Guide

This section contains comprehensive documentation for operators and administrators who want to deploy and manage CLS Backend in production environments.

## Deployment Options

- **[Kubernetes](kubernetes.md)** - Complete Kubernetes deployment guide
- **[API Gateway](api-gateway.md)** - Google Cloud API Gateway deployment
- **[Configuration](configuration.md)** - Environment variables and settings
- **[Monitoring](monitoring.md)** - Observability and alerting setup

## Quick Deployment

### Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -f deploy/kubernetes/

# Verify deployment
kubectl get pods -n cls-system
kubectl port-forward service/cls-backend 8080:80 -n cls-system
curl http://localhost:8080/health
```

### API Gateway

```bash
# Deploy with Google Cloud API Gateway
export PROJECT_ID="your-gcp-project"
export BACKEND_SERVICE_URL="https://your-backend-service-url"

./scripts/deploy-api-gateway.sh

# Test API with Google OAuth2 authentication
export ACCESS_TOKEN=$(gcloud auth print-access-token)
export USER_EMAIL=$(gcloud config get-value core/account)

curl -H "X-User-Email: $USER_EMAIL" \
     -H "Authorization: Bearer $ACCESS_TOKEN" \
     "https://your-gateway-url/api/v1/clusters"
```

## Architecture Overview

CLS Backend uses a **simplified single-tenant architecture** optimized for:

- **Clean API Design**: Simple `/api/v1/clusters` endpoints
- **External Authorization Ready**: Integration points for future authorization systems
- **Fan-Out Events**: Controller-agnostic Pub/Sub architecture
- **Binary State Reconciliation**: Simple 30s/5m intervals
- **Kubernetes-like Status**: Rich status conditions and phases

## Prerequisites

### Infrastructure Requirements

- **Kubernetes 1.20+** for container orchestration
- **PostgreSQL 13+** for data persistence with JSONB support
- **Google Cloud Pub/Sub** for event messaging
- **Google Cloud Service Account** for GCP integration

### Resource Requirements

#### Minimum (Development/Testing)
- **CPU**: 0.5 cores
- **Memory**: 512 MB
- **Storage**: 1 GB
- **Replicas**: 1

#### Recommended (Production)
- **CPU**: 2 cores
- **Memory**: 2 GB
- **Storage**: 10 GB
- **Replicas**: 3+ for high availability

#### High-Traffic (Enterprise)
- **CPU**: 4 cores
- **Memory**: 4 GB
- **Storage**: 50 GB
- **Replicas**: 5+ with horizontal pod autoscaling

## Configuration Overview

### Required Environment Variables

```bash
# Database
DATABASE_URL=postgres://user:pass@host:5432/database

# Google Cloud
GOOGLE_CLOUD_PROJECT=your-gcp-project-id

# Pub/Sub (simplified fan-out architecture)
PUBSUB_CLUSTER_EVENTS_TOPIC=cluster-events
```

### Authentication Settings

```bash
# Development (disable auth for testing)
DISABLE_AUTH=true

# Production (enable auth and external authorization)
DISABLE_AUTH=false
# External authorization system provides X-User-Email header
```

### Optional Settings

```bash
# Server
PORT=8080
ENVIRONMENT=production

# Database Connection Pool
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=5

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Reconciliation (binary state system)
RECONCILIATION_ENABLED=true
RECONCILIATION_CHECK_INTERVAL=1m
```

## Simplified Architecture Features

### Fan-Out Event System

**Single Topic Architecture:**
```
Cluster Change → cluster-events topic → All Controllers (self-filter)
```

**Benefits:**
- ✅ **Zero Maintenance**: No hardcoded controller lists
- ✅ **Auto-Discovery**: New controllers work immediately
- ✅ **Simplified Logic**: Single event per cluster change
- ✅ **Controller Independence**: Self-filtering based on platform

### Binary State Reconciliation

**Two States Only:**
- **Needs Attention**: 30-second intervals (new clusters, error states)
- **Stable**: 5-minute intervals (operational clusters)

**Benefits:**
- ✅ **90% Less Code**: Dramatically simplified from complex adaptive system
- ✅ **Easy to Debug**: Simple binary logic
- ✅ **Same Performance**: Fast response to issues, efficient for stable clusters

### External Authorization Ready

**Integration Points:**
- **User Context**: `created_by` field tracks cluster ownership
- **Request Headers**: Expects `X-User-Email` from external authorization
- **Future Multi-Tenancy**: Database schema ready for organization-based isolation
- **API Gateway Ready**: Designed for external authentication/authorization

## Security Considerations

### Production Security

- **Authentication**: Enable authentication in production (`DISABLE_AUTH=false`)
- **API Gateway**: Use external API Gateway for authentication/authorization
- **TLS**: Enable TLS for all database connections
- **Secrets Management**: Store all sensitive data in Kubernetes secrets
- **Service Accounts**: Use least-privilege GCP service accounts

### Network Security

- **Network Policies**: Implement pod-to-pod communication restrictions
- **Ingress Controllers**: Use TLS termination at ingress
- **Service Mesh**: Consider service mesh for additional security
- **Firewall Rules**: Restrict database and Pub/Sub access

### Data Security

- **Encryption at Rest**: Enable database encryption
- **Encryption in Transit**: Use TLS for all communications
- **Secret Rotation**: Regularly rotate service account keys
- **Audit Logging**: Enable audit logs for compliance

## Monitoring and Health Checks

### Health Endpoints

```bash
# Basic health check
curl http://localhost:8080/health

# Service information with version
curl http://localhost:8080/api/v1/info

# Database and Pub/Sub health
curl http://localhost:8080/health | jq '.components'
```

### Metrics

Prometheus metrics available on port 8081:
- **HTTP Metrics**: Request duration, status codes, throughput
- **Database Metrics**: Connection pool usage, query duration
- **Pub/Sub Metrics**: Message publishing, subscription lag
- **Application Metrics**: Cluster count, reconciliation timing

### Logging

**Structured JSON Logging:**
- Configurable log levels (debug, info, warn, error)
- Request correlation IDs for tracing
- Centralized log aggregation ready
- Security event logging

## Deployment Patterns

### Blue-Green Deployment

```bash
# Deploy new version to green environment
kubectl apply -f deploy/kubernetes/ --namespace=cls-system-green

# Test green environment
kubectl port-forward service/cls-backend 8080:80 -n cls-system-green

# Switch traffic (update load balancer)
# Cleanup blue environment after verification
```

### Rolling Updates

```bash
# Update image with zero downtime
kubectl set image deployment/cls-backend \
  cls-backend=gcr.io/project/cls-backend:v1.1.0 \
  -n cls-system

# Monitor rollout
kubectl rollout status deployment/cls-backend -n cls-system
```

### Canary Deployment

```bash
# Deploy canary with 10% traffic
kubectl apply -f deploy/kubernetes/canary/

# Monitor metrics and errors
# Gradually increase traffic percentage
# Complete rollout or rollback based on metrics
```

## Troubleshooting

### Common Issues

1. **Pod Startup Failures**: Check secrets, environment variables, and image pull
2. **Database Connection**: Verify DATABASE_URL and network connectivity
3. **Pub/Sub Authentication**: Check service account permissions
4. **High Memory Usage**: Monitor connection pool settings
5. **API Errors**: Check authentication headers and request format

### Debugging Commands

```bash
# Check pod logs
kubectl logs -f deployment/cls-backend -n cls-system

# Debug pod networking
kubectl exec -it deployment/cls-backend -n cls-system -- /bin/sh

# Check secrets
kubectl get secrets -n cls-system
kubectl describe secret cls-backend-secrets -n cls-system

# Monitor resources
kubectl top pods -n cls-system
kubectl describe deployment cls-backend -n cls-system
```

### Performance Troubleshooting

```bash
# Check metrics
curl http://localhost:8081/metrics | grep cls_

# Database performance
kubectl logs deployment/cls-backend -n cls-system | grep "database"

# Memory usage
kubectl top pod -n cls-system --sort-by=memory
```

## Backup and Recovery

### Database Backup

```bash
# PostgreSQL backup
pg_dump "$DATABASE_URL" > cls-backend-backup.sql

# Restore
psql "$DATABASE_URL" < cls-backend-backup.sql
```

### Configuration Backup

```bash
# Export Kubernetes resources
kubectl get all,secrets,configmaps -n cls-system -o yaml > cls-backend-k8s-backup.yaml
```

## Scaling

### Horizontal Scaling

```bash
# Manual scaling
kubectl scale deployment cls-backend --replicas=5 -n cls-system

# Horizontal Pod Autoscaler
kubectl apply -f deploy/kubernetes/hpa.yaml
kubectl get hpa -n cls-system
```

### Vertical Scaling

```bash
# Update resource requests/limits
kubectl patch deployment cls-backend -n cls-system -p '
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "cls-backend",
          "resources": {
            "requests": {"cpu": "1000m", "memory": "2Gi"},
            "limits": {"cpu": "2000m", "memory": "4Gi"}
          }
        }]
      }
    }
  }
}'
```

## Next Steps

1. **Deploy**: Follow the [Kubernetes deployment guide](kubernetes.md)
2. **Configure**: Set up [environment variables](configuration.md)
3. **Monitor**: Implement [monitoring and alerting](monitoring.md)
4. **Use**: Review the [User Guide](../user-guide/) for API usage
5. **Develop**: Check the [Developer Guide](../developer-guide/) for customization

For production deployments, ensure you have proper authentication, monitoring, and backup procedures in place.

## Related Documentation

### For Users
- **[User Guide](../user-guide/)** - API usage and common patterns
- **[Quick Start](../user-guide/quick-start.md)** - Get started in 5 minutes
- **[Examples](../user-guide/examples.md)** - Real-world API usage scenarios
- **[Troubleshooting](../user-guide/troubleshooting.md)** - Common issues and solutions

### For Developers
- **[Developer Guide](../developer-guide/)** - Contributing and development
- **[Architecture Overview](../developer-guide/architecture.md)** - System design
- **[Local Setup](../developer-guide/local-setup.md)** - Development environment
- **[Testing](../developer-guide/testing.md)** - Testing strategies

---
**Quick Links**: [📖 Documentation Home](../README.md) | [☸️ Kubernetes Setup](kubernetes.md) | [📊 Monitoring](monitoring.md)