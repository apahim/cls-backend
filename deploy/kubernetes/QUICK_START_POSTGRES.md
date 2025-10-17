# Quick Start: CLS Backend with In-Cluster PostgreSQL

This guide provides a quick deployment of cls-backend with in-cluster PostgreSQL.

## Prerequisites
- GKE cluster ready
- kubectl configured
- GCP service account key file

## Quick Deployment Steps

### 1. Create Namespace and ConfigMap
```bash
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml
```

### 2. Deploy PostgreSQL
```bash
kubectl apply -f postgres.yaml
kubectl wait --for=condition=available deployment/postgres -n cls-system --timeout=300s
```

### 3. Create Secrets
```bash
# Create minimal secrets (PostgreSQL credentials are in postgres-secret)
kubectl create secret generic cls-backend-secrets \
  --from-literal=GOOGLE_CLOUD_PROJECT="apahim-dev-1" \
  --namespace=cls-system

# Create GCP service account key (replace with your key file)
kubectl create secret generic cls-backend-gcp-key \
  --from-file=key.json=/path/to/your-service-account-key.json \
  --namespace=cls-system
```

### 4. Deploy ServiceAccount and Migration
```bash
kubectl apply -f serviceaccount.yaml
kubectl apply -f migration-job.yaml
kubectl wait --for=condition=complete job/cls-backend-migration -n cls-system --timeout=300s
```

### 5. Deploy Application and Services
```bash
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

### 6. Verify Deployment
```bash
# Check all pods are running
kubectl get pods -n cls-system

# Test health endpoint
kubectl port-forward service/cls-backend 8080:80 -n cls-system &
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/clusters
```

## File Summary

### Updated Files:
- ✅ `postgres.yaml` - In-cluster PostgreSQL with secrets
- ✅ `deployment.yaml` - Updated to use `gcr.io/apahim-dev-1/cls-backend:build-fix-20251017-114056`
- ✅ `migration-job.yaml` - Updated to use new container image
- ✅ `secret.yaml` - Updated with PostgreSQL connection string
- ✅ `DEPLOYMENT_INSTRUCTIONS.md` - Complete deployment guide

### Key Features:
- **Database**: PostgreSQL 15 with 20Gi persistent storage
- **Credentials**: Managed via kubernetes secrets
- **Application**: Latest container with compilation fixes
- **Security**: Non-root containers, security contexts
- **Monitoring**: Health checks and metrics endpoints

### Database Connection:
- **Host**: `postgres` (in-cluster service)
- **Database**: `cls_backend`
- **User**: `cls_user`
- **Password**: `cls_secure_password_2025` (change for production)
- **Connection String**: `postgres://cls_user:cls_secure_password_2025@postgres:5432/cls_backend?sslmode=disable`

## Production Notes
- Change PostgreSQL password in `postgres.yaml`
- Set `DISABLE_AUTH=false` in `configmap.yaml` for production
- Consider using external PostgreSQL (Cloud SQL) for production
- Set up proper monitoring and backups

---
**Image**: `gcr.io/apahim-dev-1/cls-backend:build-fix-20251017-114056`
**Status**: ✅ Ready for deployment to your GKE cluster