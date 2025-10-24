# CLS Backend Cloud Resources Helm Chart

This Helm chart deploys the Google Cloud resources required for the CLS Backend using Google Config Connector.

## Prerequisites

- Google Kubernetes Engine (GKE) cluster with [Config Connector](https://cloud.google.com/config-connector/docs/overview) installed
- [External Secrets Operator (ESO)](https://external-secrets.io/) installed for password generation
- Appropriate IAM permissions to create Cloud SQL instances, Pub/Sub topics, and service accounts

## Resources Created

This chart creates the following Google Cloud resources:

1. **GCP Service Enablement** - Automatically enables required GCP APIs
2. **Cloud SQL PostgreSQL Instance** - Database for CLS Backend
3. **Cloud SQL Database** - `cls_backend` database within the instance
4. **Cloud SQL User** - `cls_user` with access to the database
5. **Pub/Sub Topic** - `cluster-events` topic for event publishing
6. **IAM Service Account** - Service account for the CLS Backend application
7. **IAM Policy Bindings** - Required permissions for Pub/Sub and Cloud SQL access
8. **ESO Password Generator** - Cryptographically secure random password generation
9. **Kubernetes Secret** - Database password stored in Kubernetes Secret (via ESO)

## Installation

### 1. Install Config Connector (if not already installed)

```bash
# Enable Config Connector on your GKE cluster
gcloud container clusters update CLUSTER_NAME \
    --workload-pool=PROJECT_ID.svc.id.goog \
    --zone=ZONE

# Install Config Connector
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/k8s-config-connector/master/install-bundles/install-bundle-workload-identity/0-cnrm-system.yaml
```

### 2. Configure Values

Create a `values.yaml` file with your project configuration:

```yaml
gcp:
  project: "your-gcp-project-id"
  region: "us-central1"

# Optional: Customize database configuration
database:
  instance:
    name: "cls-backend-db"
    tier: "db-custom-2-4096"  # 2 vCPU, 4GB RAM
    diskSize: "50"            # 50GB

# Optional: Customize Pub/Sub topic name
pubsub:
  clusterEventsTopic: "cluster-events"

# Optional: Customize service account name
serviceAccount:
  name: "cls-backend"
```

### 3. Install the Chart

```bash
# Install cloud resources
helm install cls-backend-cloud-resources ./deploy/helm-cloud-resources \
  --values values.yaml \
  --namespace config-connector \
  --create-namespace
```

## Values Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gcp.project` | **Required** GCP Project ID | `""` |
| `gcp.region` | GCP region for resources | `"us-central1"` |
| `database.instance.name` | Cloud SQL instance name | `"cls-backend-db"` |
| `database.instance.tier` | Cloud SQL instance tier | `"db-custom-1-3840"` |
| `database.instance.diskSize` | Disk size in GB | `"20"` |
| `database.instance.diskType` | Disk type | `"PD_SSD"` |
| `database.instance.version` | PostgreSQL version | `"POSTGRES_15"` |
| `database.database.name` | Database name | `"cls_backend"` |
| `database.user.name` | Database username | `"cls_user"` |
| `database.user.passwordSecret.name` | Kubernetes secret name (via ESO) | `"cls-backend-db-password"` |
| `pubsub.clusterEventsTopic` | Pub/Sub topic name | `"cluster-events"` |
| `serviceAccount.name` | Service account name | `"cls-backend"` |
| `services.enabled` | Enable GCP APIs via Config Connector | `true` |

## Post-Installation

After installation, you can verify the resources were created:

```bash
# Check Cloud SQL instance
gcloud sql instances list --project=YOUR_PROJECT_ID

# Check Pub/Sub topic
gcloud pubsub topics list --project=YOUR_PROJECT_ID

# Check service account
gcloud iam service-accounts list --project=YOUR_PROJECT_ID
```

## Connection Information

After deployment, use these values for the application chart:

- **Database Instance**: Use the value from `database.instance.name`
- **Database Name**: `cls_backend`
- **Database User**: `cls_user`
- **Pub/Sub Topic**: Use the value from `pubsub.clusterEventsTopic`
- **Service Account**: `{serviceAccount.name}@{gcp.project}.iam.gserviceaccount.com`

## Important Notes

⚠️ **Security Considerations**:
- Database passwords are generated using External Secrets Operator (ESO) with cryptographically secure randomness
- Passwords are stored in Kubernetes Secrets and automatically synchronized between database and application
- Review IAM permissions before deploying to production

⚠️ **Cost Considerations**:
- Cloud SQL instances incur costs even when not in use
- Consider the appropriate instance tier for your workload
- Enable deletion protection in production

## Password Management with ESO

This chart uses External Secrets Operator (ESO) for secure database password management:

### **How it works:**
1. **Password Generator** creates a cryptographically secure 32-character password
2. **ExternalSecret** syncs the password to a Kubernetes Secret
3. **SQLUser** and application both reference the same Kubernetes Secret
4. Password remains stable across deployments (no regeneration)

### **Password Rotation:**
To rotate the database password:

```bash
# Delete the ExternalSecret to trigger new password generation
kubectl delete externalsecret cls-backend-db-password -n cls-system

# ESO will automatically:
# 1. Generate a new random password
# 2. Create a new Kubernetes Secret
# 3. SQLUser will update the database password
# 4. Application pods will restart with the new password
```

### **Benefits:**
- ✅ **No ArgoCD sync issues** - Eliminates immutable field conflicts
- ✅ **Truly random passwords** - Cryptographically secure generation
- ✅ **Automatic coordination** - Database and application use same secret
- ✅ **GitOps friendly** - No template regeneration issues

## Troubleshooting

### Config Connector Issues

```bash
# Check Config Connector status
kubectl get pods -n cnrm-system

# Check resource status
kubectl get sqlinstance,sqldatabase,sqluser,pubsubtopic,iamserviceaccount -n config-connector
```

### Permission Issues

Ensure your GKE cluster's service account has the necessary permissions:

```bash
# Grant Config Connector permissions
gcloud projects add-iam-policy-binding PROJECT_ID \
    --member="serviceAccount:cnrm-system@PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/owner"
```

## Uninstallation

```bash
# Uninstall the chart (this will delete all created resources)
helm uninstall cls-backend-cloud-resources --namespace cls-system
```

⚠️ **Warning**: This will permanently delete your Cloud SQL instance and all data. Make sure to backup your data before uninstalling.
