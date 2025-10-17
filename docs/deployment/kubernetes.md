# Kubernetes Deployment

This guide provides comprehensive instructions for deploying CLS Backend to Kubernetes clusters with the simplified single-tenant architecture and Workload Identity for secure GCP integration.

## Prerequisites

### Required Infrastructure

- **Kubernetes 1.20+** cluster with RBAC enabled
- **GKE cluster with Workload Identity enabled** (recommended for GCP)
- **PostgreSQL 13+** database (managed or self-hosted)
- **Google Cloud Pub/Sub** with enabled APIs
- **Google Cloud Service Account** configured for Workload Identity
- **Container Registry** access (GCR or Artifact Registry)

### Required Tools

- `kubectl` configured for your cluster
- `gcloud` CLI for Google Cloud operations
- `docker` or `podman` for container operations
- Access to container registry

## Step-by-Step Deployment

### 1. Prepare Container Image

#### Build and Push Image

```bash
# Set your project ID
export PROJECT_ID="your-gcp-project"

# Build for linux/amd64 (required for most Kubernetes nodes)
REGISTRY_AUTH_FILE=/path/to/.config/containers/auth.json \
  podman build --platform linux/amd64 \
  -t gcr.io/${PROJECT_ID}/cls-backend:error-handling-fix-$(date +%Y%m%d-%H%M%S) .

# Authenticate with GCR using registry auth file
REGISTRY_AUTH_FILE=/path/to/.config/containers/auth.json \
  podman login gcr.io

# Push to registry
REGISTRY_AUTH_FILE=/path/to/.config/containers/auth.json \
  podman push gcr.io/${PROJECT_ID}/cls-backend:error-handling-fix-$(date +%Y%m%d-%H%M%S)
```

**Current Recommended Image**: `gcr.io/${PROJECT_ID}/cls-backend:error-handling-fix-20251017-122651`

**Features in this image**:
- ✅ **Fixed error handling** - Returns 409 Conflict for duplicate cluster names instead of 500 errors
- ✅ **PostgreSQL constraint violations** properly detected and converted to appropriate HTTP status codes
- ✅ **Client isolation** with user email authentication
- ✅ **Simplified single-tenant architecture**
- ✅ **Fan-out Pub/Sub architecture**
- ✅ **Kubernetes-like status structures**

#### Verify Image

```bash
# List images in registry
gcloud container images list --repository=gcr.io/${PROJECT_ID}

# Verify image details
gcloud container images describe gcr.io/${PROJECT_ID}/cls-backend:latest
```

### 2. Setup Google Cloud Resources

#### Enable Required APIs

```bash
# Enable Pub/Sub API
gcloud services enable pubsub.googleapis.com --project=${PROJECT_ID}

# Verify APIs are enabled
gcloud services list --enabled --project=${PROJECT_ID} | grep pubsub
```

#### Setup Workload Identity (Recommended)

**For GKE with Workload Identity enabled**:

```bash
# Create Google Cloud service account
gcloud iam service-accounts create cls-backend \
  --display-name="CLS Backend Service Account" \
  --project=${PROJECT_ID}

# Grant Pub/Sub permissions
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:cls-backend@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/pubsub.editor"

# Allow Kubernetes service account to impersonate Google Cloud service account
gcloud iam service-accounts add-iam-policy-binding \
  cls-backend@${PROJECT_ID}.iam.gserviceaccount.com \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:${PROJECT_ID}.svc.id.goog[cls-system/cls-backend]" \
  --project=${PROJECT_ID}
```

**Alternative: Service Account Key (Less Secure)**:

```bash
# Only use if Workload Identity is not available
# Create service account
gcloud iam service-accounts create cls-backend-service \
  --display-name="CLS Backend Service Account" \
  --project=${PROJECT_ID}

# Generate key file
gcloud iam service-accounts keys create /tmp/cls-backend-key.json \
  --iam-account=cls-backend-service@${PROJECT_ID}.iam.gserviceaccount.com \
  --project=${PROJECT_ID}

# Grant Pub/Sub permissions
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:cls-backend-service@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/pubsub.admin"
```

#### Create Pub/Sub Resources

```bash
# Create cluster events topic (fan-out architecture)
gcloud pubsub topics create cluster-events --project=${PROJECT_ID}

# Verify topic creation
gcloud pubsub topics list --project=${PROJECT_ID}
```

### 3. Prepare Database

#### PostgreSQL Setup (Cloud SQL Example)

```bash
# Create Cloud SQL instance
gcloud sql instances create cls-backend-db \
  --database-version=POSTGRES_13 \
  --tier=db-g1-small \
  --region=us-central1 \
  --project=${PROJECT_ID}

# Create database
gcloud sql databases create cls_backend \
  --instance=cls-backend-db \
  --project=${PROJECT_ID}

# Create user
gcloud sql users create cls_user \
  --instance=cls-backend-db \
  --password=secure-password \
  --project=${PROJECT_ID}

# Get connection string
export DATABASE_URL="postgres://cls_user:secure-password@INSTANCE_IP:5432/cls_backend?sslmode=require"
```

#### Self-Hosted PostgreSQL

```bash
# Deploy PostgreSQL to Kubernetes (development only)
kubectl apply -f deploy/kubernetes/postgres.yaml

# For production, use managed PostgreSQL service
```

### 4. Deploy to Kubernetes

#### Create Namespace

```bash
# Apply namespace configuration
kubectl apply -f deploy/kubernetes/namespace.yaml

# Verify namespace
kubectl get namespaces | grep cls-system
```

#### Create Secrets

**For Workload Identity (Recommended)**:

```bash
# Create minimal secrets (no service account key needed)
kubectl create secret generic cls-backend-secrets \
  --from-literal=GOOGLE_CLOUD_PROJECT="${PROJECT_ID}" \
  --namespace=cls-system

# For in-cluster PostgreSQL, the postgres-secret is created automatically via postgres.yaml
# For external PostgreSQL, create database URL secret separately:
# kubectl create secret generic postgres-secret \
#   --from-literal=DATABASE_URL="${DATABASE_URL}" \
#   --namespace=cls-system

# Verify secrets
kubectl get secrets -n cls-system
```

**For Service Account Key (Alternative)**:

```bash
# Create database and GCP project secret
kubectl create secret generic cls-backend-secrets \
  --from-literal=DATABASE_URL="${DATABASE_URL}" \
  --from-literal=GOOGLE_CLOUD_PROJECT="${PROJECT_ID}" \
  --namespace=cls-system

# Create GCP service account key secret
kubectl create secret generic cls-backend-gcp-key \
  --from-file=key.json=/tmp/cls-backend-key.json \
  --namespace=cls-system

# Verify secrets
kubectl get secrets -n cls-system
```

#### Apply Configuration

```bash
# Create ConfigMap with application settings
kubectl apply -f deploy/kubernetes/configmap.yaml

# Create ServiceAccount
kubectl apply -f deploy/kubernetes/serviceaccount.yaml

# Verify configuration
kubectl get configmap,serviceaccount -n cls-system
```

#### Deploy Application

```bash
# Update deployment image reference
sed -i "s|gcr.io/PROJECT_ID|gcr.io/${PROJECT_ID}|g" deploy/kubernetes/deployment.yaml

# Update to use the latest error-handling-fix image
sed -i "s|cls-backend:.*|cls-backend:error-handling-fix-20251017-122651|g" deploy/kubernetes/deployment.yaml

# Deploy application
kubectl apply -f deploy/kubernetes/deployment.yaml

# Create service (ClusterIP for testing, LoadBalancer for external access)
kubectl apply -f deploy/kubernetes/service.yaml

# Optional: Create LoadBalancer service for external access
# kubectl apply -f deploy/kubernetes/loadbalancer-service.yaml

# Wait for deployment to be ready
kubectl wait --for=condition=available --timeout=300s \
  deployment/cls-backend -n cls-system
```

#### Run Database Migrations

```bash
# Update migration job image reference
sed -i "s|gcr.io/PROJECT_ID|gcr.io/${PROJECT_ID}|g" deploy/kubernetes/migration-job.yaml

# Run migrations
kubectl apply -f deploy/kubernetes/migration-job.yaml

# Check migration job status
kubectl get jobs -n cls-system
kubectl logs job/cls-backend-migration -n cls-system
```

### 5. Verify Deployment

#### Check Pod Status

```bash
# Check all resources
kubectl get all -n cls-system

# Check pod logs
kubectl logs -f deployment/cls-backend -n cls-system

# Check specific pod details
kubectl describe pod -l app=cls-backend -n cls-system
```

#### Test Health Endpoints

```bash
# Port forward to local machine
kubectl port-forward service/cls-backend 8080:80 -n cls-system &

# Test health check
curl http://localhost:8080/health

# Test API info
curl http://localhost:8080/api/v1/info

# Test cluster API (simplified single-tenant)
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters

# Test error handling (should return 409 Conflict for duplicates)
curl -X POST -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{"name": "test-cluster", "target_project_id": "test"}' \
  http://localhost:8080/api/v1/clusters

# Attempt to create duplicate (should return 409 instead of 500)
curl -v -X POST -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{"name": "test-cluster", "target_project_id": "test"}' \
  http://localhost:8080/api/v1/clusters
```

### Error Handling Verification

The latest image includes improved error handling:

```bash
# Expected response for duplicate cluster names:
# HTTP/1.1 409 Conflict
# {"error":{"type":"conflict","code":"CLUSTER_NAME_EXISTS","message":"A cluster with this name already exists"}}

# Test client isolation (different users can use same name):
curl -X POST -H "Content-Type: application/json" \
  -H "X-User-Email: user2@example.com" \
  -d '{"name": "test-cluster", "target_project_id": "test"}' \
  http://localhost:8080/api/v1/clusters
# Should succeed with 201 Created
```

## Configuration Management

### Environment Variables

The application is configured through environment variables in the ConfigMap:

```yaml
# deploy/kubernetes/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cls-backend-config
  namespace: cls-system
data:
  # Server settings
  PORT: "8080"
  ENVIRONMENT: "production"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"

  # Authentication (set to false for production)
  DISABLE_AUTH: "false"

  # Pub/Sub settings (simplified fan-out)
  PUBSUB_CLUSTER_EVENTS_TOPIC: "cluster-events"

  # Database connection pool
  DATABASE_MAX_OPEN_CONNS: "25"
  DATABASE_MAX_IDLE_CONNS: "5"

  # Reconciliation (binary state system)
  RECONCILIATION_ENABLED: "true"
  RECONCILIATION_CHECK_INTERVAL: "1m"
```

### Sensitive Configuration

Sensitive values are stored in Kubernetes secrets:

```yaml
# Created via kubectl create secret
apiVersion: v1
kind: Secret
metadata:
  name: cls-backend-secrets
  namespace: cls-system
type: Opaque
data:
  DATABASE_URL: <base64-encoded-database-url>
  GOOGLE_CLOUD_PROJECT: <base64-encoded-project-id>
```

### Service Account Configuration

**For Workload Identity (Recommended)**:

```yaml
# deploy/kubernetes/serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cls-backend
  namespace: cls-system
  labels:
    app.kubernetes.io/name: cls-backend
    app.kubernetes.io/component: serviceaccount
  annotations:
    # For Google Cloud Workload Identity
    iam.gke.io/gcp-service-account: cls-backend@PROJECT_ID.iam.gserviceaccount.com
```

**Benefits of Workload Identity**:
- ✅ No service account keys to manage
- ✅ Automatic credential rotation
- ✅ Follows principle of least privilege
- ✅ Better security posture
- ✅ No volume mounts needed for authentication

**For Service Account Key (Alternative)**:

```yaml
# deploy/kubernetes/serviceaccount.yaml - with volume mounts
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cls-backend
  namespace: cls-system
# No Workload Identity annotation needed
```

## Resource Management

### Resource Requests and Limits

```yaml
# In deployment.yaml
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

### Horizontal Pod Autoscaler

```yaml
# deploy/kubernetes/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: cls-backend-hpa
  namespace: cls-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: cls-backend
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

Apply HPA:
```bash
kubectl apply -f deploy/kubernetes/hpa.yaml
kubectl get hpa -n cls-system
```

## Monitoring and Observability

### Health Checks

The deployment includes health checks:

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

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

## Service Configuration

### Main Application Service

**ClusterIP Service (Recommended for testing and internal access)**:

```yaml
# deploy/kubernetes/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: cls-backend
  namespace: cls-system
  labels:
    app.kubernetes.io/name: cls-backend
    app.kubernetes.io/component: api-server
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 80
    targetPort: http
    protocol: TCP
  selector:
    app.kubernetes.io/name: cls-backend
    app.kubernetes.io/component: api-server
```

**LoadBalancer Service (For external access)**:

```yaml
# deploy/kubernetes/loadbalancer-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: cls-backend-lb
  namespace: cls-system
  labels:
    app.kubernetes.io/name: cls-backend
    app.kubernetes.io/component: loadbalancer
  annotations:
    # Allow traffic from Google Cloud API Gateway ranges
    cloud.google.com/load-balancer-type: "External"
    service.beta.kubernetes.io/load-balancer-source-ranges: "130.211.0.0/22,35.191.0.0/16"
spec:
  type: LoadBalancer
  ports:
  - name: http
    port: 80
    targetPort: http
    protocol: TCP
  selector:
    app.kubernetes.io/name: cls-backend
    app.kubernetes.io/component: api-server
```

### Metrics Collection

```yaml
# Service for metrics scraping
apiVersion: v1
kind: Service
metadata:
  name: cls-backend-metrics
  namespace: cls-system
  labels:
    app.kubernetes.io/name: cls-backend
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
    app.kubernetes.io/name: cls-backend
    app.kubernetes.io/component: api-server
```

### Log Aggregation

Configure log forwarding to your log aggregation system:

```yaml
# Example annotation for Fluentd
metadata:
  annotations:
    fluentd.kubernetes.io/log-format: json
```

## Security Configuration

### Network Policies

```yaml
# deploy/kubernetes/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cls-backend-network-policy
  namespace: cls-system
spec:
  podSelector:
    matchLabels:
      app: cls-backend
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-system
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to: []  # Allow all egress for database and Pub/Sub
    ports:
    - protocol: TCP
      port: 5432  # PostgreSQL
    - protocol: TCP
      port: 443   # HTTPS (Pub/Sub)
```

### Pod Security Context

```yaml
# In deployment.yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1001
  runAsGroup: 1001
  fsGroup: 1001
  seccompProfile:
    type: RuntimeDefault
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
```

## Backup and Disaster Recovery

### Database Backup

```bash
# Create backup job
kubectl create job cls-backend-backup \
  --from=cronjob/postgres-backup \
  -n cls-system

# Manual backup
kubectl run backup-pod --rm -i --tty \
  --image=postgres:13 \
  --env="PGPASSWORD=password" \
  -- pg_dump -h postgres-host -U username dbname > backup.sql
```

### Configuration Backup

```bash
# Export all Kubernetes resources
kubectl get all,secrets,configmaps,pvc -n cls-system -o yaml > cls-backend-k8s-backup.yaml

# Backup secrets separately (encrypted)
kubectl get secrets -n cls-system -o yaml > cls-backend-secrets-backup.yaml
```

## Troubleshooting

### Common Issues

#### 1. Image Pull Errors

```bash
# Check if image exists
gcloud container images describe gcr.io/${PROJECT_ID}/cls-backend:latest

# Check node access to registry
kubectl describe pod -l app=cls-backend -n cls-system | grep -A 5 "Failed to pull image"

# Update image pull secret if needed
kubectl create secret docker-registry gcr-json-key \
  --docker-server=gcr.io \
  --docker-username=_json_key \
  --docker-password="$(cat /tmp/cls-backend-key.json)" \
  --namespace=cls-system
```

#### 2. Database Connection Issues

```bash
# Test database connectivity
kubectl run db-test --rm -i --tty \
  --image=postgres:13 \
  --env="PGPASSWORD=password" \
  -- psql -h database-host -U username -d dbname

# Check database secret
kubectl get secret cls-backend-secrets -n cls-system -o yaml | \
  grep DATABASE_URL | base64 -d
```

#### 3. Pub/Sub Authentication

```bash
# Check service account key
kubectl get secret cls-backend-gcp-key -n cls-system -o yaml

# Test Pub/Sub access
kubectl run pubsub-test --rm -i --tty \
  --image=google/cloud-sdk:slim \
  --env="GOOGLE_APPLICATION_CREDENTIALS=/tmp/key.json" \
  -- gcloud pubsub topics list --project=${PROJECT_ID}
```

#### 4. Pod Startup Issues

```bash
# Check pod events
kubectl describe pod -l app=cls-backend -n cls-system

# Check logs
kubectl logs -l app=cls-backend -n cls-system --previous

# Check resource constraints
kubectl top pods -n cls-system
kubectl describe nodes | grep -A 5 "Allocated resources"
```

### Performance Issues

#### 1. High Memory Usage

```bash
# Monitor memory usage
kubectl top pods -n cls-system --sort-by=memory

# Check for memory leaks in logs
kubectl logs -l app=cls-backend -n cls-system | grep -i "memory\|oom"

# Adjust memory limits
kubectl patch deployment cls-backend -n cls-system -p '
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "cls-backend",
          "resources": {
            "limits": {"memory": "2Gi"}
          }
        }]
      }
    }
  }
}'
```

#### 2. High CPU Usage

```bash
# Monitor CPU usage
kubectl top pods -n cls-system --sort-by=cpu

# Check reconciliation frequency
kubectl logs -l app=cls-backend -n cls-system | grep reconciliation

# Scale up if needed
kubectl scale deployment cls-backend --replicas=5 -n cls-system
```

## Upgrade Procedures

### Rolling Update

```bash
# Update image tag
kubectl set image deployment/cls-backend \
  cls-backend=gcr.io/${PROJECT_ID}/cls-backend:v1.1.0 \
  -n cls-system

# Monitor rollout
kubectl rollout status deployment/cls-backend -n cls-system

# Rollback if needed
kubectl rollout undo deployment/cls-backend -n cls-system
```

### Database Migration

```bash
# Run migrations before deploying new version
kubectl apply -f deploy/kubernetes/migration-job.yaml

# Check migration status
kubectl logs job/cls-backend-migration -n cls-system

# Deploy new version after successful migration
kubectl set image deployment/cls-backend \
  cls-backend=gcr.io/${PROJECT_ID}/cls-backend:v1.1.0 \
  -n cls-system
```

## Production Checklist

Before deploying to production, ensure:

- [ ] **Resource Limits**: Appropriate CPU and memory limits set
- [ ] **Health Checks**: Liveness and readiness probes configured
- [ ] **Monitoring**: Metrics collection and alerting setup
- [ ] **Security**: Network policies and security contexts applied
- [ ] **Backup**: Database backup procedures in place
- [ ] **Secrets**: All sensitive data stored in Kubernetes secrets
- [ ] **Authentication**: External authorization configured
- [ ] **Scaling**: HPA configured for traffic fluctuations
- [ ] **Logging**: Log aggregation and retention configured
- [ ] **Testing**: Load testing completed
- [ ] **Documentation**: Runbooks and procedures documented

This Kubernetes deployment provides a robust, scalable foundation for running CLS Backend in production environments with the simplified single-tenant architecture.