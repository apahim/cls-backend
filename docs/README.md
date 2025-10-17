# CLS Backend Documentation

Welcome to the CLS Backend documentation! This service provides a simplified single-tenant cluster lifecycle management API with external authorization integration points.

## Getting Started

**New to CLS Backend?** Start here:

1. **[Quick Start](user-guide/quick-start.md)** - Get up and running in 5 minutes
2. **[API Usage](user-guide/api-usage.md)** - Learn common API patterns
3. **[Local Setup](developer-guide/local-setup.md)** - Set up your development environment

## Documentation Sections

### 👤 User Guide
*For users who want to interact with the CLS Backend API*

- **[User Guide Overview](user-guide/README.md)** - Getting started with the API
- **[Quick Start](user-guide/quick-start.md)** - 5-minute setup and first cluster
- **[API Usage](user-guide/api-usage.md)** - Common patterns and examples
- **[Examples](user-guide/examples.md)** - Real-world API usage scenarios
- **[NodePools](user-guide/nodepools.md)** - Managing compute node groups
- **[Troubleshooting](user-guide/troubleshooting.md)** - Solutions to common issues

### 🚀 Deployment Guide
*For operators deploying and managing CLS Backend*

- **[Deployment Overview](deployment/README.md)** - Production deployment guide
- **[Kubernetes](deployment/kubernetes.md)** - Complete Kubernetes deployment
- **[API Gateway](deployment/api-gateway.md)** - Google Cloud API Gateway setup
- **[Configuration](deployment/configuration.md)** - Environment variables and settings
- **[Monitoring](deployment/monitoring.md)** - Health checks and observability

### 👩‍💻 Developer Guide
*For developers contributing to or extending CLS Backend*

- **[Developer Overview](developer-guide/README.md)** - Development workflow and setup
- **[Local Setup](developer-guide/local-setup.md)** - Complete development environment
- **[Architecture](developer-guide/architecture.md)** - System design and components
- **[Testing](developer-guide/testing.md)** - Running tests and best practices
- **[API Development](developer-guide/api-development.md)** - Adding new endpoints
- **[Build Process](developer-guide/build-process.md)** - Container builds and CI/CD
- **[Event Architecture](developer-guide/event-architecture.md)** - Pub/Sub fan-out design
- **[Status System](developer-guide/status-system.md)** - Kubernetes-like status

### 📚 Reference
*Complete API and technical reference*

- **[API Reference](reference/api.md)** - Complete REST API specification
- **[OpenAPI](reference/openapi.md)** - OpenAPI/Swagger specification

## Quick Navigation

### Common Tasks

| I want to... | Go to... |
|--------------|----------|
| Create my first cluster | [Quick Start](user-guide/quick-start.md) |
| Deploy to production | [Kubernetes Deployment](deployment/kubernetes.md) |
| Set up development environment | [Local Setup](developer-guide/local-setup.md) |
| Understand the architecture | [Architecture Overview](developer-guide/architecture.md) |
| Add a new API endpoint | [API Development](developer-guide/api-development.md) |
| Troubleshoot issues | [Troubleshooting](user-guide/troubleshooting.md) |

### API Quick Reference

```bash
# Health check
GET /health

# Clusters (simplified single-tenant)
GET    /api/v1/clusters              # List clusters
POST   /api/v1/clusters              # Create cluster
GET    /api/v1/clusters/{id}         # Get cluster
PUT    /api/v1/clusters/{id}         # Update cluster
DELETE /api/v1/clusters/{id}         # Delete cluster
GET    /api/v1/clusters/{id}/status  # Get status
```

**Authentication**: All requests require `X-User-Email` header (except in development mode with `DISABLE_AUTH=true`)

## Architecture Highlights

CLS Backend uses a **simplified single-tenant architecture** with:

- **Clean API Design**: Simple `/api/v1/clusters` endpoints
- **External Authorization Ready**: Integration points for future authorization systems
- **Fan-Out Events**: Controller-agnostic Pub/Sub architecture
- **Binary State Reconciliation**: Simple 30s/5m intervals
- **Kubernetes-like Status**: Rich status conditions and phases

## Key Technologies

- **Go 1.21+** - Primary programming language
- **Gin Framework** - HTTP server and routing
- **PostgreSQL 13+** - Database with JSONB support
- **Google Cloud Pub/Sub** - Event messaging
- **Kubernetes** - Deployment and orchestration

## Getting Help

- **[Troubleshooting Guide](user-guide/troubleshooting.md)** - Common issues and solutions
- **[GitHub Issues](https://github.com/your-org/cls-backend/issues)** - Report bugs and request features
- **[API Reference](reference/api.md)** - Complete API documentation

## Contributing

Interested in contributing? Start with:

1. **[Local Setup](developer-guide/local-setup.md)** - Set up your development environment
2. **[Architecture Overview](developer-guide/architecture.md)** - Understand the system design
3. **[Testing Guide](developer-guide/testing.md)** - Learn our testing practices
4. **[API Development](developer-guide/api-development.md)** - Add new features

---

**Quick Links**: [User Guide](user-guide/) | [Deployment](deployment/) | [Developer Guide](developer-guide/) | [API Reference](reference/api.md)