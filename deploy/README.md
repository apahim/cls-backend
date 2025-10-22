# CLS Backend Deployment Guide

Simple deployment guide for the CLS Backend using three automated Helm charts.

## 📦 Charts

1. **helm-cloud-resources/** - Creates GCP infrastructure (Cloud SQL, Pub/Sub, IAM)
2. **helm-application/** - Deploys Kubernetes application with auto-discovery
3. **helm-api-gateway/** - Creates public API Gateway with OAuth2 (optional)

## 🚀 Quick Deploy

### Prerequisites

- GKE cluster with Config Connector
- Helm 3.x installed
- GCP APIs enabled:
  ```bash
  gcloud services enable container.googleapis.com sqladmin.googleapis.com \
    pubsub.googleapis.com apigateway.googleapis.com secretmanager.googleapis.com
  ```

### 1. Deploy Cloud Resources

```bash
cd helm-cloud-resources/

# Configure your project
cat > values-prod.yaml << EOF
gcp:
  project: "your-gcp-project-id"

database:
  instance:
    name: "cls-backend-db"
    tier: "db-custom-1-3840"

serviceAccount:
  name: "cls-backend"

workloadIdentity:
  enabled: true
  kubernetesNamespace: "cls-system"
  kubernetesServiceAccount: "cls-backend"
EOF

# Deploy
helm install cls-cloud-resources . -f values-prod.yaml
```

**Wait**: ~10-15 minutes for Cloud SQL to be ready.

### 2. Deploy Application

```bash
cd ../helm-application/

# Minimal configuration (everything auto-discovered!)
cat > values-prod.yaml << EOF
gcp:
  project: "your-gcp-project-id"

image:
  repository: "gcr.io/your-project/cls-backend"
  tag: "latest"

namespace:
  name: "cls-system"
  create: true
EOF

# Deploy with automation
helm install cls-application . -f values-prod.yaml
```

**🤖 Auto-Discovery**: Database config, service accounts, and Pub/Sub topics are automatically discovered from the cloud-resources chart!

### 3. Deploy API Gateway (Optional)

```bash
cd ../helm-api-gateway/

# Configure API Gateway (IP auto-discovered!)
cat > values-prod.yaml << EOF
gcp:
  project: "your-gcp-project-id"

# backend.externalIP auto-discovered from LoadBalancer service

oauth2:
  clientId: "your-oauth2-client-id.apps.googleusercontent.com"

cors:
  allowedOrigins:
    - "https://console.redhat.com"
    - "https://hybrid-cloud-console.redhat.com"
EOF

# Deploy
helm install cls-api-gateway . -f values-prod.yaml
```

## ✅ Verify Deployment

```bash
# Check cloud resources
kubectl get sqlinstance,pubsubtopic,iamserviceaccount -n config-connector

# Check application
kubectl get pods,svc -n cls-system

# Test health
kubectl port-forward service/cls-application 8080:80 -n cls-system &
curl http://localhost:8080/health
```

## 🤖 Automation Benefits

The charts now automatically handle:

- ✅ **Database configuration** - Auto-discovered from Cloud SQL instances
- ✅ **Service account names** - Auto-discovered via Workload Identity
- ✅ **Pub/Sub topics** - Auto-discovered from cloud resources
- ✅ **LoadBalancer IPs** - Auto-discovered for API Gateway
- ✅ **Cross-chart consistency** - No manual parameter coordination needed

## 🔧 Manual Overrides (Optional)

You can still manually specify values if needed:

```yaml
# helm-application/values.yaml
database:
  instanceName: "custom-db-name"    # Override auto-discovery
  databaseName: "custom_db"
  username: "custom_user"

serviceAccount:
  gcpServiceAccountName: "custom-sa"

pubsub:
  clusterEventsTopic: "custom-events"
```

## 🐛 Troubleshooting

### Cloud SQL not ready
```bash
kubectl get sqlinstance -n config-connector
gcloud sql instances describe cls-backend-db
```

### Application pods failing
```bash
kubectl logs deployment/cls-application -n cls-system
kubectl describe pod -l app.kubernetes.io/name=cls-backend -n cls-system
```

### Auto-discovery not working
```bash
# Check Config Connector resources exist
kubectl get sqlinstance,iamserviceaccount,pubsubtopic -n config-connector

# Check if lookup is finding resources
helm template cls-application ./helm-application -f values-prod.yaml | grep -A5 -B5 "DATABASE_URL"
```

### LoadBalancer IP pending
```bash
kubectl get service cls-application-external -n cls-system
kubectl describe service cls-application-external -n cls-system
```

## 🗑️ Cleanup

```bash
# Remove in reverse order
helm uninstall cls-api-gateway
helm uninstall cls-application
helm uninstall cls-cloud-resources
```

## 📁 Chart Details

| Chart | Purpose | Resources Created |
|-------|---------|------------------|
| **cloud-resources** | GCP Infrastructure | Cloud SQL, Pub/Sub, IAM Service Account, Secret Manager |
| **application** | Kubernetes App | Deployment, Services, ConfigMap, Secret, Migration Job |
| **api-gateway** | Public API | API Gateway, OAuth2 Config, CORS |

**Deployment Order**: cloud-resources → application → api-gateway

---

**🚀 That's it!** The automation handles the rest.