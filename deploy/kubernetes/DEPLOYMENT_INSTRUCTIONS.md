# CLS Backend Deployment Instructions

## Container Image Updated
**Ready for Deployment**: `gcr.io/apahim-dev-1/cls-backend:error-handling-fix-20251017-122651`

## Prerequisites

### 1. GKE Cluster Requirements
- **Kubernetes Version**: 1.25+ recommended
- **Node Pool**: Minimum 3 nodes for HA deployment
- **Workload Identity**: Enabled (recommended for GCP integration)
- **Network**: Private cluster with authorized networks (recommended)

### 2. Required GCP APIs
Ensure these APIs are enabled in your project:
```bash
gcloud services enable container.googleapis.com
gcloud services enable pubsub.googleapis.com
gcloud services enable sqladmin.googleapis.com
```

## Deployment Options

### Option A: In-Cluster PostgreSQL (Recommended for Testing)
This deploys PostgreSQL inside the Kubernetes cluster using the provided `postgres.yaml`.

### Option B: External PostgreSQL (Recommended for Production)
Use Google Cloud SQL or external managed PostgreSQL.

## Deployment Steps

### Step 1: Choose Database Option

#### Option A: In-Cluster PostgreSQL Setup
This option deploys PostgreSQL inside your Kubernetes cluster:
- Uses `postgres.yaml` for deployment
- 20Gi persistent volume
- Credentials managed via `postgres-secret`
- Suitable for development and testing

#### Option B: External Database Setup (Production)
```bash
# Create PostgreSQL instance (or use existing)
gcloud sql instances create cls-backend-db \
  --database-version=POSTGRES_15 \
  --tier=db-custom-2-4096 \
  --region=us-central1

# Create database
gcloud sql databases create cls_backend --instance=cls-backend-db

# Create database user
gcloud sql users create cls_user --instance=cls-backend-db --password=<secure-password>
```

#### Pub/Sub Topics
```bash
# Create required Pub/Sub topic
gcloud pubsub topics create cluster-events
```

#### Service Account (Optional - for Workload Identity)
```bash
# Create service account
gcloud iam service-accounts create cls-backend-sa \
  --display-name="CLS Backend Service Account"

# Grant required permissions
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:cls-backend-sa@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.editor"

gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:cls-backend-sa@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client"
```

### Step 2: Configure Secrets

#### For In-Cluster PostgreSQL (Option A)
The PostgreSQL credentials are managed automatically via `postgres-secret` in `postgres.yaml`.
You only need to create the GCP service account secret:

```bash
# Create GCP service account key secret
kubectl create secret generic cls-backend-gcp-key \
  --from-file=key.json=/path/to/service-account-key.json \
  --namespace=cls-system

# Create minimal cls-backend secrets (only GCP project)
kubectl create secret generic cls-backend-secrets \
  --from-literal=GOOGLE_CLOUD_PROJECT="apahim-dev-1" \
  --namespace=cls-system
```

#### For External PostgreSQL (Option B)
```bash
# Create secrets manually with external database URL
kubectl create secret generic cls-backend-secrets \
  --from-literal=DATABASE_URL="postgres://cls_user:password@external-host:5432/cls_backend?sslmode=require" \
  --from-literal=GOOGLE_CLOUD_PROJECT="apahim-dev-1" \
  --namespace=cls-system

# Create GCP service account key secret
kubectl create secret generic cls-backend-gcp-key \
  --from-file=key.json=/path/to/service-account-key.json \
  --namespace=cls-system

# Skip postgres.yaml deployment when using external database
```

#### Alternative: Update secret.yaml (Template provided)
Edit `secret.yaml` with your actual values and apply:
```bash
kubectl apply -f secret.yaml  # For external database
# OR
kubectl apply -f secret-postgres-integrated.yaml  # For in-cluster PostgreSQL
```

### Step 3: Deploy Application

#### Deploy in Order:

**For In-Cluster PostgreSQL (Option A):**
```bash
# 1. Create namespace
kubectl apply -f namespace.yaml

# 2. Create ConfigMap
kubectl apply -f configmap.yaml

# 3. Deploy PostgreSQL first
kubectl apply -f postgres.yaml

# 4. Wait for PostgreSQL to be ready
kubectl wait --for=condition=available deployment/postgres -n cls-system --timeout=300s

# 5. Create Secrets (minimal for in-cluster postgres)
kubectl create secret generic cls-backend-secrets \
  --from-literal=GOOGLE_CLOUD_PROJECT="apahim-dev-1" \
  --namespace=cls-system

kubectl create secret generic cls-backend-gcp-key \
  --from-file=key.json=/path/to/service-account-key.json \
  --namespace=cls-system

# 6. Create ServiceAccount and RBAC
kubectl apply -f serviceaccount.yaml

# 7. Run Database Migrations
kubectl apply -f migration-job.yaml

# Wait for migration to complete
kubectl wait --for=condition=complete job/cls-backend-migration -n cls-system --timeout=300s

# 8. Deploy Application
kubectl apply -f deployment.yaml

# 9. Create Services
kubectl apply -f service.yaml
kubectl apply -f loadbalancer-service.yaml  # If external access needed

# 10. Optional: Create Ingress
kubectl apply -f ingress.yaml  # If using ingress
```

**For External PostgreSQL (Option B):**
```bash
# 1. Create namespace
kubectl apply -f namespace.yaml

# 2. Create ConfigMap
kubectl apply -f configmap.yaml

# 3. Create Secrets (with external database URL)
kubectl apply -f secret.yaml  # Edit with your external DB details first

kubectl create secret generic cls-backend-gcp-key \
  --from-file=key.json=/path/to/service-account-key.json \
  --namespace=cls-system

# 4. Create ServiceAccount and RBAC
kubectl apply -f serviceaccount.yaml

# 5. Run Database Migrations
kubectl apply -f migration-job.yaml

# Wait for migration to complete
kubectl wait --for=condition=complete job/cls-backend-migration -n cls-system --timeout=300s

# 6. Deploy Application
kubectl apply -f deployment.yaml

# 7. Create Services
kubectl apply -f service.yaml
kubectl apply -f loadbalancer-service.yaml  # If external access needed

# 8. Optional: Create Ingress
kubectl apply -f ingress.yaml  # If using ingress
```

### Step 4: Verify Deployment

#### Check All Components
```bash
# Check all pods
kubectl get pods -n cls-system

# Check services
kubectl get svc -n cls-system

# Check persistent volumes (for in-cluster PostgreSQL)
kubectl get pvc -n cls-system
```

#### Verify PostgreSQL (for In-Cluster Option)
```bash
# Check PostgreSQL logs
kubectl logs deployment/postgres -n cls-system

# Test PostgreSQL connection
kubectl exec -it deployment/postgres -n cls-system -- psql -U cls_user -d cls_backend -c "SELECT version();"

# Check database tables after migration
kubectl exec -it deployment/postgres -n cls-system -- psql -U cls_user -d cls_backend -c "\dt"
```

#### Verify cls-backend Application
```bash
# Check application logs
kubectl logs -l app.kubernetes.io/name=cls-backend -n cls-system

# Test health endpoint
kubectl port-forward service/cls-backend 8080:80 -n cls-system
curl http://localhost:8080/health

# Test API endpoints
curl http://localhost:8080/api/v1/info
curl http://localhost:8080/api/v1/clusters
```

## Configuration Details

### Current Image Configuration
- **Image**: `gcr.io/apahim-dev-1/cls-backend:error-handling-fix-20251017-122651`
- **Features**:
  - ✅ Build fixes applied (compilation errors resolved)
  - ✅ Client isolation with user email authentication
  - ✅ Simplified single-tenant architecture
  - ✅ Fan-out Pub/Sub architecture
  - ✅ Kubernetes-like status structures
  - ✅ Health checks and metrics endpoints
  - ✅ **Fixed error handling** - Returns 409 Conflict for duplicate cluster names instead of 500 errors
  - ✅ **PostgreSQL constraint violations** properly detected and converted to appropriate HTTP status codes

### Resource Allocation
- **Replicas**: 3 (High Availability)
- **CPU**: 100m request, 500m limit
- **Memory**: 128Mi request, 512Mi limit
- **Ports**: 8080 (API), 8081 (Metrics)

### Environment Variables
Key configurations in ConfigMap:
- `ENVIRONMENT=production`
- `DISABLE_AUTH=true` (Testing mode - change for production)
- `PUBSUB_CLUSTER_EVENTS_TOPIC=cluster-events`
- `RECONCILIATION_ENABLED=true`

## Security Considerations

### Production Readiness Checklist
- [ ] **Database**: Use Cloud SQL with private IP
- [ ] **Authentication**: Set `DISABLE_AUTH=false` and configure proper auth
- [ ] **Network**: Deploy in private subnet with authorized networks
- [ ] **Secrets**: Use Google Secret Manager or Workload Identity
- [ ] **RBAC**: Review and minimize service account permissions
- [ ] **TLS**: Configure TLS termination at ingress/load balancer
- [ ] **Monitoring**: Set up logging and monitoring alerts

### Network Security
- Service uses Internal LoadBalancer by default
- Health check configured for API Gateway integration
- Pod security context enforces non-root user (1001)
- Read-only root filesystem enabled

## Troubleshooting

### Common Issues

#### PostgreSQL Issues
1. **PostgreSQL Pod Crashes**:
   - Check PVC is bound: `kubectl get pvc -n cls-system`
   - Verify storage class availability: `kubectl get storageclass`
   - Check PostgreSQL logs: `kubectl logs deployment/postgres -n cls-system`

2. **PostgreSQL Connection Refused**:
   - Wait for PostgreSQL readiness: `kubectl wait --for=condition=available deployment/postgres -n cls-system --timeout=300s`
   - Check PostgreSQL service: `kubectl get svc postgres -n cls-system`
   - Test connection: `kubectl exec -it deployment/postgres -n cls-system -- pg_isready`

3. **Database Authentication Errors**:
   - Verify postgres-secret: `kubectl get secret postgres-secret -n cls-system -o yaml`
   - Check if credentials match between postgres-secret and cls-backend connection

#### Application Issues
1. **Migration Job Fails**:
   - Check database connectivity: `kubectl logs job/cls-backend-migration -n cls-system`
   - Verify DATABASE_URL in secrets
   - Ensure PostgreSQL is ready before running migrations

2. **cls-backend Pods Crash**:
   - Verify ConfigMap and Secret values
   - Check if DATABASE_URL is accessible from cls-backend pod
   - Test: `kubectl exec -it deployment/cls-backend -n cls-system -- env | grep DATABASE_URL`

3. **503 Errors**:
   - Check if migration completed successfully
   - Verify all pods are ready: `kubectl get pods -n cls-system`
   - Check service endpoints: `kubectl get endpoints -n cls-system`

4. **Auth Issues**:
   - Verify GCP service account permissions
   - Check if GOOGLE_APPLICATION_CREDENTIALS file exists in pod

### Debug Commands
```bash
# Check pod events
kubectl describe pod <pod-name> -n cls-system

# Check migration job logs
kubectl logs job/cls-backend-migration -n cls-system

# Check application logs
kubectl logs deployment/cls-backend -n cls-system

# Check service endpoints
kubectl get endpoints -n cls-system
```

## API Endpoints

Once deployed, the service provides:

### Health & Status
- `GET /health` - Service health check
- `GET /api/v1/info` - API version info

### Cluster Management
- `GET /api/v1/clusters` - List clusters
- `POST /api/v1/clusters` - Create cluster
- `GET /api/v1/clusters/{id}` - Get cluster details
- `PUT /api/v1/clusters/{id}` - Update cluster
- `DELETE /api/v1/clusters/{id}` - Delete cluster
- `GET /api/v1/clusters/{id}/status` - Get cluster status

### Metrics
- `GET /metrics` (port 8081) - Prometheus metrics

## Next Steps

1. **Deploy to GKE Cluster**: Follow the deployment steps above
2. **Configure Monitoring**: Set up Prometheus/Grafana monitoring
3. **Enable Authentication**: Configure proper authentication for production
4. **Test API**: Verify all endpoints work correctly
5. **Deploy Controllers**: Deploy cluster controllers that will consume events

---
**Last Updated**: 2025-10-17
**Image Tag**: `error-handling-fix-20251017-122651`
**Status**: ✅ Ready for Deployment - **Error Handling Fixed**