# Developer Guide

This section contains comprehensive documentation for developers who want to contribute to, extend, or understand the CLS Backend codebase.

## Getting Started

- **[Local Setup](local-setup.md)** - Complete development environment setup
- **[Architecture Overview](architecture.md)** - System design and components
- **[API Development](api-development.md)** - Building and extending APIs

## Core Concepts

- **[Status System](status-system.md)** - Kubernetes-like status structures and aggregation
- **[Event Architecture](event-architecture.md)** - Pub/Sub fan-out design and controller integration
- **[Testing Guide](testing.md)** - Unit tests, integration tests, and best practices

## Build and Deployment

- **[Build Process](build-process.md)** - Container builds and CI/CD
- **[Database Migrations](build-process.md#database-migration)** - Schema evolution and data migrations

## For Contributors

Before contributing, please read:
- [Code Style Guidelines](../CONTRIBUTING.md#code-style)
- [Pull Request Process](../CONTRIBUTING.md#pull-requests)
- [Testing Requirements](testing.md)

## Quick Development Setup

```bash
# Clone and setup
git clone https://github.com/your-org/cls-backend.git
cd cls-backend

# Install dependencies
go mod download

# Setup test database
export DATABASE_URL="postgres://test:test@localhost:5432/cls_test"

# Run tests
make test-unit
make test-integration

# Start development server
export DISABLE_AUTH=true
go run ./cmd/backend-api
```

## Development Workflow

1. **Setup Environment**: Follow [Local Setup](local-setup.md)
2. **Understand Architecture**: Read [Architecture Overview](architecture.md)
3. **Make Changes**: Implement features following our [API Development](api-development.md) guide
4. **Test Thoroughly**: Use our [Testing Guide](testing.md)
5. **Submit PR**: Follow [Contributing Guidelines](../CONTRIBUTING.md)

## Key Technologies

- **Go 1.21+** - Primary programming language
- **Gin Framework** - HTTP server and routing
- **PostgreSQL 13+** - Database with JSONB support
- **Google Cloud Pub/Sub** - Event messaging
- **Kubernetes** - Deployment and orchestration
- **Docker/Podman** - Containerization

## Simplified Architecture Highlights

CLS Backend uses a **simplified single-tenant architecture** with:

- **Clean API Design**: Simple `/api/v1/clusters` endpoints
- **External Authorization Ready**: Integration points for future authorization systems
- **Fan-Out Events**: Controller-agnostic Pub/Sub architecture
- **Binary State Reconciliation**: Simple 30s/5m intervals
- **Kubernetes-like Status**: Rich status conditions and phases

## Code Organization

```
cls-backend/
├── cmd/backend-api/          # Application entry point
├── internal/
│   ├── api/                  # HTTP handlers and middleware
│   ├── database/             # Repository layer and migrations
│   ├── models/               # Data structures
│   ├── config/               # Configuration management
│   ├── pubsub/               # Event publishing and handling
│   ├── reconciliation/       # Reconciliation scheduling
│   └── utils/                # Shared utilities
├── docs/                     # Documentation
├── deploy/                   # Kubernetes manifests
└── Makefile                  # Build automation
```

## Community and Support

- **GitHub Issues**: Report bugs and request features
- **GitHub Discussions**: Ask questions and share ideas
- **Code Reviews**: All changes reviewed by maintainers
- **Documentation**: Keep docs updated with code changes

Happy coding! 🚀

## Related Documentation

### For Users
- **[User Guide](../user-guide/)** - API usage and patterns
- **[Quick Start](../user-guide/quick-start.md)** - Get started quickly
- **[Examples](../user-guide/examples.md)** - Real-world scenarios
- **[API Reference](../reference/api.md)** - Complete API specification

### For Deployment
- **[Deployment Guide](../deployment/)** - Production deployment
- **[Kubernetes Setup](../deployment/kubernetes.md)** - Complete K8s deployment
- **[Monitoring](../deployment/monitoring.md)** - Observability setup

---
**Quick Links**: [📖 Documentation Home](../README.md) | [🏠 Local Setup](local-setup.md) | [🏗️ Architecture](architecture.md) | [🧪 Testing](testing.md)