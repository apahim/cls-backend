# Contributing to CLS Backend

Thank you for your interest in contributing to CLS Backend! This document provides guidelines and information for contributors.

## Code of Conduct

This project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How to Contribute

### Reporting Issues

- Use the GitHub issue tracker to report bugs or request features
- Search existing issues before creating a new one
- Provide clear, detailed information including:
  - Steps to reproduce (for bugs)
  - Expected vs actual behavior
  - Environment details (Go version, OS, etc.)

### Development Setup

1. **Prerequisites**:
   - Go 1.21 or later
   - Docker or Podman for container builds
   - PostgreSQL for local testing
   - Google Cloud SDK (for GCP integration)

2. **Local Setup**:
   ```bash
   # Clone the repository
   git clone https://github.com/your-org/cls-backend.git
   cd cls-backend

   # Install dependencies
   go mod download

   # Build the application
   make build

   # Run tests
   make test-unit
   ```

3. **Environment Configuration**:
   ```bash
   export DATABASE_URL="postgres://user:pass@localhost:5432/cls_backend_test"
   export GOOGLE_CLOUD_PROJECT="your-test-project"
   export DISABLE_AUTH=true  # For local development
   ```

### Making Changes

1. **Fork** the repository and create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**:
   - Follow the existing code style and patterns
   - Add tests for new functionality
   - Update documentation as needed

3. **Test your changes**:
   ```bash
   # Run unit tests
   make test-unit

   # Run linter
   go vet ./...
   gofmt -l .

   # Build to ensure no compilation errors
   make build
   ```

4. **Commit your changes**:
   - Use clear, descriptive commit messages
   - Follow conventional commit format: `type(scope): description`
   - Examples:
     - `feat(api): add cluster status endpoint`
     - `fix(reconciliation): handle edge case in scheduler`
     - `docs(api): update API reference examples`

### Submitting Pull Requests

1. **Push your branch** to your fork
2. **Create a pull request** with:
   - Clear title and description
   - Reference to any related issues
   - Description of changes made
   - Screenshots or examples if applicable

3. **Address review feedback**:
   - Respond to comments promptly
   - Make requested changes
   - Push updates to the same branch

## Development Guidelines

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Write clear, self-documenting code
- Add comments for complex logic
- Keep functions focused and small

### Testing

- Write unit tests for new functionality
- Maintain test coverage above 80%
- Use table-driven tests where appropriate
- Mock external dependencies

### Documentation

- Update documentation for API changes
- Add docstrings for public functions
- Update README if changing setup process
- Include examples for new features

### Git Workflow

- Use feature branches for development
- Keep commits atomic and focused
- Rebase feature branches before merging
- Squash commits if requested during review

## Project Structure

```
cls-backend/
├── cmd/backend-api/           # Application entry point
├── internal/
│   ├── api/                   # HTTP handlers and routing
│   ├── config/                # Configuration management
│   ├── database/              # Repository layer and migrations
│   ├── models/                # Data models and types
│   ├── pubsub/                # Google Cloud Pub/Sub integration
│   ├── services/              # Business logic layer
│   └── utils/                 # Utilities and logging
├── deploy/kubernetes/         # Kubernetes manifests
├── docs/                      # Documentation
└── examples/                  # Practical examples
```

## Architecture Principles

- **Simplicity**: Prefer simple solutions over complex ones
- **Single Responsibility**: Each component has a clear purpose
- **External Authorization**: Designed for external auth integration
- **Controller Agnostic**: Fan-out architecture supports any controllers
- **Status Transparency**: Kubernetes-like status structures

## Release Process

1. **Version Tagging**: Use semantic versioning (v1.2.3)
2. **Changelog**: Update CHANGELOG.md with changes
3. **Documentation**: Ensure docs are up to date
4. **Testing**: Run full test suite
5. **Container Build**: Build and test container images

## Getting Help

- Check the [documentation](docs/)
- Search existing [issues](https://github.com/your-org/cls-backend/issues)
- Join discussions in [GitHub Discussions](https://github.com/your-org/cls-backend/discussions)
- Review the [API reference](docs/reference/api.md)

## Recognition

Contributors are recognized in:
- Git commit history
- Release notes
- GitHub contributors page

Thank you for contributing to CLS Backend!