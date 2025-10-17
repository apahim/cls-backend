# CLS Backend

A production-ready cluster lifecycle management service that provides comprehensive cluster operations with Kubernetes-like status structures and a simplified, maintainable architecture.

## Features

- **Simple Architecture**: Single-tenant design ready for external authorization integration
- **Kubernetes-like Status**: Rich status conditions, phases, and observed generation tracking
- **Controller Agnostic**: Fan-out Pub/Sub architecture with zero hardcoded controller dependencies
- **Auto-Discovery**: New controllers work immediately by creating a Pub/Sub subscription
- **Real-time Status**: Hybrid status aggregation with efficient caching
- **Production Ready**: Health checks, metrics, structured logging, and graceful shutdown

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 13+
- Google Cloud Project with Pub/Sub enabled

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/cls-backend.git
cd cls-backend

# Build the application
make build

# Set environment variables
export DATABASE_URL="postgres://user:pass@localhost:5432/cls_backend"
export GOOGLE_CLOUD_PROJECT="your-project-id"
export DISABLE_AUTH=true  # For local development

# Run the service
./bin/backend-api
```

### API Usage

```bash
# Create a cluster
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: user@example.com" \
  -d '{"name": "my-cluster", "spec": {"platform": {"type": "gcp"}}}'

# List clusters
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters

# Get cluster status
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters/{cluster-id}
```

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  External Auth  │────│   CLS Backend    │────│   PostgreSQL    │
│  (Future)       │    │  (Single Tenant) │    │   (Clusters &   │
└─────────────────┘    └──────────────────┘    │   Status Data)  │
                               │               └─────────────────┘
                       ┌──────────────────┐
                       │  Google Cloud    │──┐ Fan-Out Events
                       │    Pub/Sub       │  │
                       │ (cluster-events) │  │
                       └──────────────────┘  │
                               │             │
                    ┌──────────┴─────────────┴──────────┐
                    │                                   │
            ┌───────▼────────┐                 ┌────────▼──────┐
            │  Controller A  │                 │ Controller B  │
            │ (Self-Filters) │                 │(Self-Filters) │
            └────────────────┘                 └───────────────┘
```

**Key Benefits:**
- Controllers auto-discover via Pub/Sub subscriptions
- No hardcoded controller lists in backend
- Single reconciliation event per cluster (fan-out)
- External authorization ready via `created_by` field

## Documentation

📖 **[Complete Documentation](docs/)** - Comprehensive guides for users, developers, and operators

- **[User Guide](docs/user-guide/)** - API usage and getting started
- **[Developer Guide](docs/developer-guide/)** - Contributing and architecture
- **[Deployment Guide](docs/deployment/)** - Kubernetes deployment and configuration
- **[API Reference](docs/reference/api.md)** - Complete API documentation

**Quick Links**: [🚀 Quick Start](docs/user-guide/quick-start.md) | [🏗️ Architecture](docs/developer-guide/architecture.md) | [☸️ Kubernetes Setup](docs/deployment/kubernetes.md)

## Status System

CLS Backend provides Kubernetes-like status structures:

```json
{
  "status": {
    "observedGeneration": 1,
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "reason": "AllControllersReady",
        "message": "All controllers are operational"
      }
    ],
    "phase": "Ready"
  }
}
```

**Status Phases**: `Pending` → `Progressing` → `Ready` (or `Failed`)

## Deployment

### Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -f deploy/kubernetes/

# Verify deployment
kubectl get pods -n cls-system
kubectl port-forward service/cls-backend 8080:80 -n cls-system
curl http://localhost:8080/health
```

### Configuration

Required environment variables:
- `DATABASE_URL` - PostgreSQL connection string
- `GOOGLE_CLOUD_PROJECT` - GCP project ID for Pub/Sub

Optional:
- `DISABLE_AUTH=true` - Disable authentication for development
- `LOG_LEVEL=debug` - Set logging level

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

- **Issues**: Report bugs or request features via [GitHub Issues](https://github.com/your-org/cls-backend/issues)
- **Pull Requests**: Follow our [development guidelines](docs/developer-guide/)
- **Code of Conduct**: Please read our [Code of Conduct](CODE_OF_CONDUCT.md)

## License

This project is licensed under the [MIT License](LICENSE).

## Support

- [Documentation](docs/)
- [API Reference](docs/reference/api.md)
- [GitHub Issues](https://github.com/your-org/cls-backend/issues)
- [GitHub Discussions](https://github.com/your-org/cls-backend/discussions)