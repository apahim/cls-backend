# CLS Backend API Gateway Helm Chart

This Helm chart deploys Google API Gateway configuration for the CLS Backend with OAuth2 authentication using Google Config Connector.

## Prerequisites

- Google Kubernetes Engine (GKE) cluster with [Config Connector](https://cloud.google.com/config-connector/docs/overview) installed
- Google API Gateway API enabled (`gcloud services enable apigateway.googleapis.com`)
- Pre-provisioned external static IP address in GCP
- OAuth2 client configured in Google Cloud Console
- CLS Backend application deployed with external LoadBalancer service

## Architecture

```
Internet → Google API Gateway (OAuth2) → External LoadBalancer → CLS Backend Pods
         ↑                              ↑
         OAuth2 Authentication         Static IP via nip.io
```

## Resources Created

This chart creates the following Google Cloud resources:

1. **API Gateway API** - The API definition
2. **API Gateway API Config** - OpenAPI specification with OAuth2 and backend configuration
3. **API Gateway Gateway** - The actual gateway instance

## Installation

### 1. Create Static IP Address

First, create a static external IP address that will be used by both the LoadBalancer and API Gateway:

```bash
# Create static external IP
gcloud compute addresses create cls-backend-external-ip \
  --global \
  --ip-version IPV4 \
  --project YOUR_PROJECT_ID

# Get the IP address
export EXTERNAL_IP=$(gcloud compute addresses describe cls-backend-external-ip \
  --global --format="value(address)" --project YOUR_PROJECT_ID)

echo "External IP: $EXTERNAL_IP"
```

### 2. Create OAuth2 Client

Create an OAuth2 client in Google Cloud Console:

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Navigate to APIs & Services > Credentials
3. Create OAuth 2.0 Client ID
4. Add authorized domains (e.g., `gateway.dev`)
5. Note the Client ID for configuration

### 3. Deploy Application with External LoadBalancer

Update your application deployment to enable the external service:

```bash
# Deploy application chart (LoadBalancer service is created automatically)
helm install cls-backend-app ./deploy/helm-application \
  --values app-values.yaml
```

### 4. Configure Values

Create a `values.yaml` file for the API Gateway:

```yaml
gcp:
  project: "your-gcp-project-id"
  region: "us-central1"

# Backend configuration (REQUIRED)
backend:
  externalIP: "34.172.156.70"  # Your static IP
  port: 80
  protocol: "http"

# OAuth2 configuration (update with your client ID)
oauth2:
  clientId: "32555940559.apps.googleusercontent.com"

# Optional: Customize API Gateway names
apiGateway:
  api:
    name: "cls-backend-api"
  gateway:
    name: "cls-backend-gateway"

# Optional: Customize CORS origins
cors:
  allowedOrigins:
    - "https://console.redhat.com"
    - "https://hybrid-cloud-console.redhat.com"
```

### 5. Install API Gateway

```bash
# Install API Gateway configuration
helm install cls-backend-gateway ./deploy/helm-api-gateway \
  --values api-gateway-values.yaml \
  --namespace config-connector \
  --create-namespace
```

## Values Reference

### Required Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gcp.project` | **Required** GCP Project ID | `""` |
| `backend.externalIP` | **Required** External static IP address | `""` |

### API Gateway Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gcp.region` | GCP region for API Gateway | `"us-central1"` |
| `apiGateway.api.name` | API Gateway API name | `"cls-backend-api"` |
| `apiGateway.gateway.name` | API Gateway Gateway name | `"cls-backend-gateway"` |

### Backend Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `backend.port` | Backend service port | `80` |
| `backend.protocol` | Backend protocol | `"http"` |

### OAuth2 Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `oauth2.clientId` | Google OAuth2 Client ID | `"32555940559.apps.googleusercontent.com"` |
| `oauth2.scopes` | OAuth2 scopes | `["openid", "email", "profile"]` |

### CORS Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `cors.allowedOrigins` | Allowed CORS origins | `["https://console.redhat.com", "https://hybrid-cloud-console.redhat.com"]` |
| `cors.allowedMethods` | Allowed HTTP methods | `["GET", "POST", "PUT", "DELETE", "OPTIONS"]` |
| `cors.allowedHeaders` | Allowed headers | `["Content-Type", "Authorization", "X-User-Email", "X-Request-ID"]` |
| `cors.maxAge` | CORS max age (seconds) | `86400` |
| `cors.allowCredentials` | Allow credentials in CORS | `true` |

## Post-Installation

### Get Gateway URL

After deployment, get the gateway URL:

```bash
# Get the gateway details
gcloud api-gateway gateways describe cls-backend-gateway \
  --location=us-central1 \
  --project=YOUR_PROJECT_ID

# The gateway URL will be in the format:
# https://cls-backend-gateway-XXXXXXXX.REGION.gateway.dev
```

### Test API Gateway

```bash
# Test health endpoint (no authentication required)
curl https://cls-backend-gateway-XXXXXXXX.us-central1.gateway.dev/health

# Test authenticated endpoint (requires OAuth2 token)
curl -H "Authorization: Bearer YOUR_OAUTH2_TOKEN" \
  https://cls-backend-gateway-XXXXXXXX.us-central1.gateway.dev/api/v1/clusters
```

## Configuration Consistency

⚠️ **Important**: The external IP address must be consistent across components:

1. **Static IP**: Created in GCP with `gcloud compute addresses create`
2. **Application Chart**: `externalService.externalIP` in helm-application values
3. **API Gateway Chart**: `backend.externalIP` in helm-api-gateway values

## Authentication Flow

1. **User Access**: Users authenticate via Google OAuth2
2. **Token Validation**: API Gateway validates OAuth2 token
3. **Header Injection**: Gateway adds `X-User-Email` header
4. **Backend Call**: Request forwarded to CLS Backend with user context

## API Endpoints

The API Gateway exposes the following user-facing endpoints:

- `GET /health` - Health check (no authentication)
- `GET /api/v1/clusters` - List user's clusters
- `POST /api/v1/clusters` - Create cluster
- `GET /api/v1/clusters/{id}` - Get cluster details
- `PUT /api/v1/clusters/{id}` - Update cluster
- `DELETE /api/v1/clusters/{id}` - Delete cluster
- `GET /api/v1/clusters/{id}/status` - Get cluster status

**Note**: Controller endpoints are not exposed through the API Gateway and access the backend directly.

## Troubleshooting

### Gateway Not Created

Check Config Connector status:

```bash
kubectl get apigatewayapi,apigatewayapiconfig,apigatewaygateway -n config-connector
```

### Backend Connection Issues

Verify the external LoadBalancer service:

```bash
# Check external service
kubectl get svc cls-backend-app-external -n cls-system

# Verify external IP is assigned
kubectl get svc cls-backend-app-external -n cls-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

### OAuth2 Issues

1. Check OAuth2 client configuration in Google Cloud Console
2. Verify client ID in values.yaml
3. Ensure authorized domains include `gateway.dev`

### CORS Issues

Update CORS configuration in values.yaml:

```yaml
cors:
  allowedOrigins:
    - "https://your-frontend-domain.com"
```

## Production Considerations

### Security

1. **OAuth2 Client**: Use production OAuth2 client with restricted domains
2. **Static IP**: Use reserved static IP address
3. **CORS**: Restrict origins to your actual frontend domains
4. **Rate Limiting**: Configure rate limiting in API Gateway (not included in basic chart)

### Monitoring

1. **Gateway Metrics**: Available in Google Cloud Monitoring
2. **Access Logs**: Configure access logging for API Gateway
3. **Backend Metrics**: CLS Backend exposes Prometheus metrics

### Example Production Values

```yaml
gcp:
  project: "your-prod-project"
  region: "us-central1"

backend:
  externalIP: "34.172.156.70"  # Your production static IP

oauth2:
  clientId: "your-prod-oauth2-client-id.apps.googleusercontent.com"

cors:
  allowedOrigins:
    - "https://console.yourcompany.com"
    - "https://app.yourcompany.com"

apiGateway:
  api:
    name: "cls-backend-prod-api"
  gateway:
    name: "cls-backend-prod-gateway"
```

## Uninstallation

```bash
# Remove API Gateway configuration
helm uninstall cls-backend-gateway --namespace config-connector

# Clean up static IP (optional)
gcloud compute addresses delete cls-backend-external-ip --global
```

## Integration with Other Charts

This chart works together with:

1. **helm-cloud-resources**: Provides Cloud SQL, Pub/Sub, and IAM resources
2. **helm-application**: Provides the CLS Backend application with external LoadBalancer

Deployment order:
1. helm-cloud-resources (GCP infrastructure)
2. helm-application (LoadBalancer service created automatically)
3. helm-api-gateway (API Gateway configuration)