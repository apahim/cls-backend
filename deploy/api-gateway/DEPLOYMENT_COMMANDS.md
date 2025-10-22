# API Gateway Deployment Commands

This document contains the commands to deploy new versions of the API Gateway configuration for the cls-backend service.

## Deploy New Configuration

### Step 1: Create New API Configuration
```bash
# Set timestamp for unique config ID
export CONFIG_ID="cls-user-oauth2-$(date +%Y%m%d-%H%M%S)"

# Create new API configuration
gcloud api-gateway api-configs create $CONFIG_ID \
  --api=cls-backend-api \
  --openapi-spec=deploy/api-gateway/cls-backend-user-api-oauth2.yaml \
  --display-name="CLS User OAuth2 - $(date +%Y-%m-%d %H:%M)"
```

### Step 2: Update Gateway to Use New Configuration
```bash
# Update the gateway to use the new configuration
gcloud api-gateway gateways update cls-backend-gateway \
  --api-config=$CONFIG_ID \
  --api=cls-backend-api \
  --location=us-central1
```