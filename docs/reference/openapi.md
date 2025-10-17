# OpenAPI Specification

This document provides information about the CLS Backend OpenAPI specification for Google Cloud API Gateway integration.

## OpenAPI Specification File

**Location**: [`docs/reference/openapi-spec.yaml`](./openapi-spec.yaml)

**Format**: OpenAPI 2.0 (Swagger 2.0) - compatible with Google Cloud API Gateway

## Features

### 🔐 **Authentication Support**
- **API Keys** - For development and testing
- **OAuth 2.0** - For production user authentication
- **Service Accounts** - For server-to-server communication

### 📊 **Complete API Coverage**
- **Clusters** - Full CRUD operations with status management
- **NodePools** - Complete nodepool lifecycle management
- **Health Checks** - Public health endpoints
- **Status Endpoints** - Controller status reporting

### 🛡️ **Security Features**
- Input validation with parameter constraints
- Standardized error response formats
- Request/response schema definitions
- Security requirement specifications

### 🏗️ **Google Cloud Integration**
- `x-google-backend` extensions for backend routing
- Request timeout configurations
- Path translation settings
- Compatible with Cloud Run, GKE, and Compute Engine

## API Endpoints

### Clusters

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/clusters` | List all clusters |
| `POST` | `/api/v1/clusters` | Create a new cluster |
| `GET` | `/api/v1/clusters/{id}` | Get cluster details |
| `PUT` | `/api/v1/clusters/{id}` | Update cluster |
| `DELETE` | `/api/v1/clusters/{id}` | Delete cluster |
| `GET` | `/api/v1/clusters/{id}/status` | Get cluster status |
| `PUT` | `/api/v1/clusters/{id}/status` | Update cluster status |

### NodePools

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/nodepools` | List all nodepools |
| `POST` | `/api/v1/nodepools` | Create a new nodepool |
| `GET` | `/api/v1/nodepools/{id}` | Get nodepool details |
| `PUT` | `/api/v1/nodepools/{id}` | Update nodepool |
| `DELETE` | `/api/v1/nodepools/{id}` | Delete nodepool |
| `GET` | `/api/v1/nodepools/{id}/status` | Get nodepool status |
| `PUT` | `/api/v1/nodepools/{id}/status` | Update nodepool status |

### Public Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Service health check |

## Authentication

### Google Cloud User Authentication

All API endpoints (except `/health`) require **Google Cloud user authentication** with OAuth 2.0:

**Required Authentication:**
- **Google OAuth 2.0 Bearer Token** - Users must authenticate via `gcloud auth` or Google OAuth flow
- **X-User-Email Header** - User email for authorization and ownership tracking

```bash
# User email (required for all endpoints)
X-User-Email: user@example.com

# Google OAuth 2.0 Bearer token (required)
Authorization: Bearer YOUR_GOOGLE_ACCESS_TOKEN
```

### Getting Authentication Token

**Option 1: Using gcloud CLI (Recommended)**
```bash
# Authenticate with Google Cloud
gcloud auth login

# Get access token
export ACCESS_TOKEN=$(gcloud auth print-access-token)
```

**Option 2: Using OAuth 2.0 Flow**
- **Client ID**: `32555940559.apps.googleusercontent.com`
- **Scopes**: `openid email profile`
- **Auth URL**: `https://accounts.google.com/o/oauth2/auth`

### Example Requests

```bash
# Get access token
export ACCESS_TOKEN=$(gcloud auth print-access-token)
export USER_EMAIL=$(gcloud config get-value core/account)

# List clusters
curl -H "X-User-Email: $USER_EMAIL" \
     -H "Authorization: Bearer $ACCESS_TOKEN" \
     "https://api.example.com/api/v1/clusters"

# Create cluster
curl -X POST \
     -H "Content-Type: application/json" \
     -H "X-User-Email: $USER_EMAIL" \
     -H "Authorization: Bearer $ACCESS_TOKEN" \
     -d '{"name": "my-cluster", "spec": {"platform": {"type": "gcp"}}}' \
     "https://api.example.com/api/v1/clusters"
```

## Data Models

### Key Data Types

- **Cluster** - Main cluster resource with spec and status
- **NodePool** - Group of compute nodes within a cluster
- **ClusterSpec** - Desired cluster configuration
- **ClusterStatus** - Current cluster state with Kubernetes-like conditions
- **Condition** - Status condition with type, status, and timestamps

### Platform Support

**Supported Platforms:**
- **GCP** - Google Cloud Platform with zones, machine types, disk configuration
- **AWS** - Amazon Web Services with availability zones, instance types
- **Azure** - Microsoft Azure with regions, VM sizes, disk types

### Example Schema

```yaml
# Cluster creation request
CreateClusterRequest:
  type: object
  required: [name, spec]
  properties:
    name:
      type: string
      pattern: "^[a-z0-9-]+$"
    spec:
      $ref: "#/definitions/ClusterSpec"

# Cluster response with status
Cluster:
  type: object
  properties:
    id:
      type: string
      format: uuid
    name:
      type: string
    generation:
      type: integer
    status:
      $ref: "#/definitions/ClusterStatus"
```

## Validation Rules

### Parameter Validation

- **Cluster names**: `^[a-z0-9-]+$` (lowercase alphanumeric and hyphens)
- **Pagination**: `limit` 1-100, `offset` ≥ 0
- **UUIDs**: Valid UUID format for all ID fields
- **Email addresses**: Valid email format for user identification

### Request Size Limits

- **JSON payload**: Reasonable size limits enforced
- **Array fields**: Maximum items specified
- **String fields**: Maximum length constraints

## Error Handling

### Standard Error Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request body",
    "details": "cluster name is required"
  }
}
```

### HTTP Status Codes

- **200** - Success
- **201** - Created
- **204** - No Content (for DELETE operations)
- **400** - Bad Request (validation errors)
- **401** - Unauthorized (authentication required)
- **404** - Not Found
- **409** - Conflict (resource already exists)
- **500** - Internal Server Error

## API Gateway Deployment

### Quick Deploy

```bash
# Deploy to Google Cloud API Gateway
export PROJECT_ID="your-gcp-project"
export BACKEND_SERVICE_URL="https://your-backend-service-url"

./scripts/deploy-api-gateway.sh
```

### Manual Configuration

```bash
# Create API Gateway configuration
gcloud api-gateway api-configs create cls-backend-config \
  --api cls-backend-api \
  --openapi-spec docs/reference/openapi-spec.yaml \
  --project YOUR_PROJECT_ID
```

## Customization

### Backend URL Configuration

Update the OpenAPI spec for your backend:

```yaml
# Replace in openapi-spec.yaml
x-google-backend:
  address: "https://your-cls-backend-service.run.app"
  path_translation: "APPEND_PATH_TO_ADDRESS"
```

### Authentication Configuration

Choose your authentication method:

```yaml
# API Key authentication (development)
security:
  - api_key: []

# OAuth 2.0 authentication (production)
security:
  - oauth2: ["read", "write"]
```

### Rate Limiting

Add quota configuration:

```yaml
x-google-quota:
  metricRules:
    - selector: "*"
      metricCosts:
        "apigateway.googleapis.com/api_request": 1
```

## Development Tools

### Swagger UI

View the interactive API documentation:

```bash
# Serve locally with Swagger UI
npx swagger-ui-serve docs/reference/openapi-spec.yaml
```

### Code Generation

Generate client libraries:

```bash
# Generate Python client
swagger-codegen generate \
  -i docs/reference/openapi-spec.yaml \
  -l python \
  -o client/python

# Generate JavaScript client
swagger-codegen generate \
  -i docs/reference/openapi-spec.yaml \
  -l javascript \
  -o client/javascript
```

### Validation

Validate the OpenAPI specification:

```bash
# Using Google Cloud SDK
gcloud api-gateway api-configs validate \
  --api-config docs/reference/openapi-spec.yaml

# Using swagger-tools
swagger-tools validate docs/reference/openapi-spec.yaml
```

## Resources

- **[OpenAPI Specification](./openapi-spec.yaml)** - The complete spec file
- **[API Gateway Deployment Guide](../deployment/api-gateway.md)** - Deployment instructions
- **[User API Guide](../user-guide/api-usage.md)** - API usage examples
- **[Google API Gateway Documentation](https://cloud.google.com/api-gateway/docs)**
- **[OpenAPI 2.0 Specification](https://swagger.io/specification/v2/)**

## Support

For questions about the API specification:

1. **Issues**: Report problems in the GitHub repository
2. **Documentation**: Check the [API usage guide](../user-guide/api-usage.md)
3. **Examples**: See [example clients](../../examples/api/)
4. **Community**: Join discussions in GitHub Discussions