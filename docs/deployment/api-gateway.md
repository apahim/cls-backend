# Google Cloud API Gateway Deployment

This guide covers deploying CLS Backend behind Google Cloud API Gateway for production use with authentication, rate limiting, and monitoring.

## Overview

Google Cloud API Gateway provides:
- **Authentication & Authorization** - OAuth 2.0, API keys, service accounts
- **Rate Limiting** - Protect your backend from overload
- **Monitoring & Analytics** - Request metrics and logging
- **SSL Termination** - HTTPS endpoints with managed certificates
- **Request/Response Transformation** - Standardized error formats
- **Caching** - Improved performance and reduced backend load

## Prerequisites

1. **Google Cloud Project** with billing enabled
2. **CLS Backend** deployed (Cloud Run, GKE, Compute Engine)
3. **gcloud CLI** installed and authenticated
4. **API Gateway API** enabled in your project

## Quick Deployment

### 1. Deploy Your Backend Service

First, ensure your CLS Backend is deployed and accessible:

```bash
# Example: Deploy to Cloud Run
gcloud run deploy cls-backend \
  --source . \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --project YOUR_PROJECT_ID
```

### 2. Deploy API Gateway

Use the included deployment script:

```bash
# Set required environment variables
export PROJECT_ID="your-gcp-project"
export BACKEND_SERVICE_URL="https://cls-backend-hash-uc.a.run.app"

# Run deployment script
./scripts/deploy-api-gateway.sh
```

### 3. Test Your API

```bash
# Get your gateway URL from the deployment output
GATEWAY_URL="https://cls-backend-gateway-hash.uc.gateway.dev"

# Test health endpoint (public)
curl "$GATEWAY_URL/health"

# Test API with Google OAuth2 authentication
export ACCESS_TOKEN=$(gcloud auth print-access-token)
export USER_EMAIL=$(gcloud config get-value core/account)

curl -H "X-User-Email: $USER_EMAIL" \
     -H "Authorization: Bearer $ACCESS_TOKEN" \
     "$GATEWAY_URL/api/v1/clusters"
```

## Manual Deployment Steps

If you prefer manual deployment:

### 1. Enable Required APIs

```bash
gcloud services enable apigateway.googleapis.com \
  servicemanagement.googleapis.com \
  servicecontrol.googleapis.com \
  --project YOUR_PROJECT_ID
```

### 2. Update Configuration

Edit `docs/reference/openapi-spec.yaml`:

```yaml
# Replace placeholders
host: "YOUR_API_GATEWAY_HOST"
x-google-backend:
  address: "YOUR_BACKEND_SERVICE_URL"
```

### 3. Create API and Configuration

```bash
# Create API
gcloud api-gateway apis create cls-backend-api \
  --project YOUR_PROJECT_ID

# Create configuration
gcloud api-gateway api-configs create cls-backend-config \
  --api cls-backend-api \
  --openapi-spec docs/reference/openapi-spec.yaml \
  --project YOUR_PROJECT_ID

# Create gateway
gcloud api-gateway gateways create cls-backend-gateway \
  --api cls-backend-api \
  --api-config cls-backend-config \
  --location us-central1 \
  --project YOUR_PROJECT_ID
```

## Authentication Configuration

### Google Cloud User Authentication (Production)

The API Gateway is configured for **Google Cloud user authentication** using OAuth 2.0:

**Authentication Configuration:**
```yaml
securityDefinitions:
  google_oauth2:
    type: oauth2
    authorizationUrl: https://accounts.google.com/o/oauth2/auth
    flow: implicit
    x-google-jwks_uri: https://www.googleapis.com/oauth2/v3/certs
    x-google-audiences: "32555940559.apps.googleusercontent.com"
    scopes:
      openid: OpenID Connect scope
      email: Access to user email address
      profile: Access to user profile information
```

### User Authentication Steps

**Step 1: Authenticate with Google Cloud**
```bash
# Login to Google Cloud
gcloud auth login

# Verify authentication
gcloud auth list
```

**Step 2: Get Access Token**
```bash
# Get access token for API calls
export ACCESS_TOKEN=$(gcloud auth print-access-token)
export USER_EMAIL=$(gcloud config get-value core/account)

# Verify token (optional)
curl -H "Authorization: Bearer $ACCESS_TOKEN" \
     "https://www.googleapis.com/oauth2/v1/tokeninfo"
```

**Step 3: Use API**
```bash
# Make authenticated API calls
curl -H "X-User-Email: $USER_EMAIL" \
     -H "Authorization: Bearer $ACCESS_TOKEN" \
     "$GATEWAY_URL/api/v1/clusters"
```

### Service Account Authentication (For Automation)

For automated systems, use service account impersonation:

```bash
# Create service account
gcloud iam service-accounts create cls-backend-client \
  --display-name="CLS Backend API Client" \
  --project YOUR_PROJECT_ID

# Grant user access to impersonate service account
gcloud iam service-accounts add-iam-policy-binding \
  cls-backend-client@YOUR_PROJECT_ID.iam.gserviceaccount.com \
  --member="user:your-email@example.com" \
  --role="roles/iam.serviceAccountTokenCreator"

# Use impersonation for API calls
gcloud auth print-access-token \
  --impersonate-service-account=cls-backend-client@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

## Advanced Configuration

### Rate Limiting

Add quota configuration to your OpenAPI spec:

```yaml
x-google-quota:
  metricRules:
    - selector: "*"
      metricCosts:
        "apigateway.googleapis.com/api_request": 1
  quotaLimits:
    - name: "requests-per-minute"
      metric: "apigateway.googleapis.com/api_request"
      unit: "1/min/{project}"
      values:
        STANDARD: 1000
```

### Custom Domains

```bash
# Map custom domain
gcloud api-gateway gateways update cls-backend-gateway \
  --location us-central1 \
  --update-labels environment=production \
  --project YOUR_PROJECT_ID

# Configure DNS
# CNAME: api.yourdomain.com -> cls-backend-gateway-hash.uc.gateway.dev
```

### Request Transformation

Add request/response transformation:

```yaml
x-google-backend:
  address: YOUR_BACKEND_SERVICE_URL
  path_translation: APPEND_PATH_TO_ADDRESS
  request_timeout: 30s
```

## Monitoring and Logging

### Enable Logging

```bash
# Enable audit logging
gcloud logging sinks create cls-backend-audit-sink \
  bigquery.googleapis.com/projects/YOUR_PROJECT_ID/datasets/api_audit \
  --log-filter='protoPayload.serviceName="cls-backend-api"'
```

### Monitoring Dashboard

```bash
# Create monitoring policy
gcloud alpha monitoring policies create \
  --policy-from-file=monitoring-policy.yaml
```

Example monitoring policy:

```yaml
# monitoring-policy.yaml
displayName: "CLS Backend API Gateway"
conditions:
  - displayName: "High Error Rate"
    conditionThreshold:
      filter: 'resource.type="api"'
      comparison: COMPARISON_GREATER_THAN
      thresholdValue: 0.05
```

## Security Best Practices

### 1. Network Security

```yaml
# Restrict backend access
x-google-backend:
  address: YOUR_BACKEND_SERVICE_URL
  request_timeout: 30s
  # Only allow requests from API Gateway
```

### 2. CORS Configuration

```yaml
# Add CORS support
responses:
  200:
    headers:
      Access-Control-Allow-Origin:
        type: string
        default: "https://yourdomain.com"
```

### 3. Input Validation

```yaml
# Strict parameter validation
parameters:
  - name: limit
    in: query
    type: integer
    minimum: 1
    maximum: 100
    required: false
```

## Troubleshooting

### Common Issues

1. **502 Bad Gateway**
   ```bash
   # Check backend service health
   curl YOUR_BACKEND_SERVICE_URL/health

   # Verify API Gateway can reach backend
   gcloud logging read 'resource.type="api_gateway"' --limit=10
   ```

2. **Authentication Failures**
   ```bash
   # Check API key restrictions
   gcloud alpha services api-keys describe KEY_ID

   # Verify OAuth token
   curl -H "Authorization: Bearer TOKEN" https://www.googleapis.com/oauth2/v1/tokeninfo
   ```

3. **Rate Limiting**
   ```bash
   # Check quota usage
   gcloud logging read 'jsonPayload.quota_exceeded=true' --limit=10
   ```

### Debugging Commands

```bash
# View API Gateway logs
gcloud logging read 'resource.type="api_gateway"' \
  --format="table(timestamp, severity, jsonPayload.message)"

# Check API configuration
gcloud api-gateway api-configs describe CONFIG_ID \
  --api API_ID --format="value(gatewayConfig)"

# Monitor request metrics
gcloud monitoring metrics list \
  --filter='metric.type:starts_with("apigateway.googleapis.com")'
```

## Cost Optimization

### 1. Request Caching

```yaml
# Enable response caching
x-google-backend:
  address: YOUR_BACKEND_SERVICE_URL
  cache_key_policy:
    include_host: false
    include_protocol: false
    include_query_string: true
```

### 2. Backend Optimization

- Use Cloud Run for automatic scaling
- Implement proper health checks
- Optimize response sizes
- Use compression

### 3. Monitoring Costs

```bash
# Set up billing alerts
gcloud alpha billing budgets create \
  --billing-account BILLING_ACCOUNT_ID \
  --display-name "API Gateway Budget" \
  --budget-amount 100USD
```

## Production Checklist

- [ ] Custom domain configured
- [ ] OAuth 2.0 authentication enabled
- [ ] Rate limiting configured
- [ ] Monitoring and alerting set up
- [ ] Error logging configured
- [ ] Security headers added
- [ ] CORS policies configured
- [ ] Backend health checks working
- [ ] Backup authentication method
- [ ] Cost monitoring enabled

## Updates and Versioning

### Rolling Updates

```bash
# Create new configuration version
CONFIG_ID_V2="cls-backend-config-$(date +%Y%m%d-%H%M%S)"

gcloud api-gateway api-configs create $CONFIG_ID_V2 \
  --api cls-backend-api \
  --openapi-spec api-gateway-config-v2.yaml

# Update gateway (zero downtime)
gcloud api-gateway gateways update cls-backend-gateway \
  --api cls-backend-api \
  --api-config $CONFIG_ID_V2 \
  --location us-central1
```

### Blue-Green Deployment

```bash
# Create production gateway
gcloud api-gateway gateways create cls-backend-gateway-prod \
  --api cls-backend-api \
  --api-config $CONFIG_ID_V2 \
  --location us-central1

# Switch DNS when ready
# Update DNS: api.yourdomain.com -> new-gateway-url
```

For more information, see:
- [Google Cloud API Gateway Documentation](https://cloud.google.com/api-gateway/docs)
- [OpenAPI 2.0 Specification](https://swagger.io/specification/v2/)
- [CLS Backend API Reference](../reference/api.md)