# CLS Backend Application Helm Chart

This Helm chart deploys the CLS Backend application to Kubernetes.

## Prerequisites

- Kubernetes cluster (GKE recommended for Workload Identity)
- Google Cloud resources deployed using the `helm-cloud-resources` chart
- Container image available in a registry (GCR/Artifact Registry)

## Resources Created

This chart creates the following Kubernetes resources:

1. **Namespace** - Application namespace (`cls-system`)
2. **ServiceAccount** - Kubernetes service account with Workload Identity annotations
3. **ConfigMap** - Non-sensitive application configuration
4. **Secret** - Sensitive configuration (database URL, GCP project)
5. **Deployment** - CLS Backend application pods (3 replicas by default)
6. **Service** - ClusterIP service for internal access

## Installation

### 1. Prerequisites Check

Ensure the cloud resources are deployed first:

```bash
# Verify Cloud SQL instance exists
gcloud sql instances describe cls-backend-db --project=YOUR_PROJECT_ID

# Verify Pub/Sub topic exists
gcloud pubsub topics describe cluster-events --project=YOUR_PROJECT_ID

# Verify service account exists
gcloud iam service-accounts describe cls-backend@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

### 2. Configure Workload Identity (GKE)

```bash
# Enable Workload Identity on your GKE cluster (if not already enabled)
gcloud container clusters update CLUSTER_NAME \
    --workload-pool=PROJECT_ID.svc.id.goog \
    --zone=ZONE

# Allow the Kubernetes service account to impersonate the Google service account
gcloud iam service-accounts add-iam-policy-binding \
    cls-backend@PROJECT_ID.iam.gserviceaccount.com \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:PROJECT_ID.svc.id.goog[cls-system/cls-backend]"
```

### 3. Configure Values

Create a `values.yaml` file:

```yaml
gcp:
  project: "your-gcp-project-id"

# Container image (update with your image)
image:
  repository: "gcr.io/your-project/cls-backend"
  tag: "latest"

# Database configuration (must match cloud-resources chart)
database:
  instanceName: "cls-backend-db"
  databaseName: "cls_backend"
  username: "cls_user"
  password: "cls_secure_password_2024"

# Pub/Sub configuration (must match cloud-resources chart)
pubsub:
  clusterEventsTopic: "cluster-events"

# Service account configuration (must match cloud-resources chart)
serviceAccount:
  gcpServiceAccountName: "cls-backend"
```

### 4. Install the Chart

```bash
# Install the application
helm install cls-backend-app ./deploy/helm-application \
  --values values.yaml \
  --create-namespace
```

## Values Reference

### Required Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gcp.project` | **Required** GCP Project ID | `""` |

### Application Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `namespace.name` | Kubernetes namespace | `"cls-system"` |
| `namespace.create` | Create namespace | `true` |
| `image.repository` | Container image repository | `"gcr.io/apahim-dev-1/cls-backend"` |
| `image.tag` | Container image tag | `"latest"` |
| `image.pullPolicy` | Image pull policy | `Always` |

### Database Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `database.instanceName` | Cloud SQL instance name | `"cls-backend-db"` |
| `database.databaseName` | Database name | `"cls_backend"` |
| `database.username` | Database username | `"cls_user"` |
| `database.password` | Database password | `"cls_secure_password_2024"` |

### Pub/Sub Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `pubsub.clusterEventsTopic` | Pub/Sub topic name | `"cluster-events"` |

### Service Account Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.name` | Kubernetes service account name | `"cls-backend"` |
| `serviceAccount.gcpServiceAccountName` | GCP service account name | `"cls-backend"` |
| `serviceAccount.workloadIdentity` | Enable Workload Identity | `true` |

### Deployment Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `deployment.replicas` | Number of replicas | `3` |
| `resources.requests.memory` | Memory request | `"128Mi"` |
| `resources.requests.cpu` | CPU request | `"100m"` |
| `resources.limits.memory` | Memory limit | `"512Mi"` |
| `resources.limits.cpu` | CPU limit | `"500m"` |

### Application Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.port` | HTTP server port | `8080` |
| `config.environment` | Environment name | `"production"` |
| `config.logLevel` | Log level | `"info"` |
| `config.disableAuth` | Disable authentication | `false` |
| `config.metricsEnabled` | Enable metrics | `true` |
| `config.metricsPort` | Metrics port | `8081` |

## Post-Installation

### Verify Deployment

```bash
# Check pods
kubectl get pods -n cls-system

# Check service
kubectl get svc -n cls-system

# Check logs
kubectl logs -f deployment/cls-backend-app -n cls-system
```

### Access the Application

```bash
# Port forward to access locally
kubectl port-forward service/cls-backend-app 8080:80 -n cls-system

# Test health endpoint
curl http://localhost:8080/health

# Test API endpoints
curl http://localhost:8080/api/v1/clusters
```

## Configuration Consistency

⚠️ **Important**: Values in this chart must match those used in the `helm-cloud-resources` chart:

- `database.instanceName` → Cloud SQL instance name
- `database.databaseName` → Cloud SQL database name
- `database.username` → Cloud SQL user name
- `database.password` → Cloud SQL user password
- `pubsub.clusterEventsTopic` → Pub/Sub topic name
- `serviceAccount.gcpServiceAccountName` → GCP service account name

## Monitoring

The application exposes Prometheus metrics on port 8081:

```bash
# Access metrics
kubectl port-forward service/cls-backend-app 8081:8081 -n cls-system
curl http://localhost:8081/metrics
```

## Troubleshooting

### Database Connection Issues

```bash
# Check database connectivity
kubectl exec -it deployment/cls-backend-app -n cls-system -- /bin/sh
# Inside the pod:
# pg_isready -h cls-backend-db -p 5432 -U cls_user
```

### Pub/Sub Issues

```bash
# Check service account permissions
gcloud projects get-iam-policy YOUR_PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:cls-backend@YOUR_PROJECT_ID.iam.gserviceaccount.com"
```

### Workload Identity Issues

```bash
# Verify Workload Identity binding
gcloud iam service-accounts get-iam-policy \
  cls-backend@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

### Pod Logs

```bash
# Check application logs
kubectl logs -f deployment/cls-backend-app -n cls-system

# Check previous logs if pod restarted
kubectl logs deployment/cls-backend-app -n cls-system --previous
```

## Scaling

```bash
# Scale replicas
kubectl scale deployment cls-backend-app --replicas=5 -n cls-system

# Or update values.yaml and upgrade
helm upgrade cls-backend-app ./deploy/helm-application --values values.yaml
```

## Upgrades

```bash
# Upgrade with new image
helm upgrade cls-backend-app ./deploy/helm-application \
  --set image.tag=v1.2.0 \
  --values values.yaml
```

## Uninstallation

```bash
# Uninstall the application
helm uninstall cls-backend-app
```

Note: This only removes the Kubernetes resources. The Google Cloud resources (database, Pub/Sub topic, etc.) remain and should be removed separately using the cloud resources chart.
