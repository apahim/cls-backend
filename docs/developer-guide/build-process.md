# Build Process

This document explains the complete build process for CLS Backend, including local builds, container builds, and deployment workflows.

## Overview

CLS Backend uses a multi-stage build process optimized for:
- **Fast local development** with hot reloading
- **Optimized container builds** for production deployment
- **Multi-platform support** for different architectures
- **Automated CI/CD** integration

## Local Development Builds

### Quick Local Build

```bash
# Build binary locally
make build

# Build and run
make run

# Build with specific flags
go build -o bin/backend-api ./cmd/backend-api
```

### Development with Hot Reload

```bash
# Install air for hot reloading
go install github.com/cosmtrek/air@latest

# Run with auto-restart on code changes
air

# Or use go run for simple cases
go run ./cmd/backend-api
```

### Testing Builds

```bash
# Run all tests
make test

# Run only unit tests
make test-unit

# Run with coverage
go test -cover ./internal/...

# Test specific package
go test -v ./internal/api
```

## Container Builds

### Multi-Stage Dockerfile

CLS Backend uses a sophisticated multi-stage build:

```dockerfile
# Stage 1: Builder (golang:1.21-alpine)
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -a -installsuffix cgo \
    -o backend-api \
    ./cmd/backend-api

# Stage 2: Runtime (alpine:3.18)
FROM alpine:3.18
RUN apk --no-cache add ca-certificates tzdata curl
RUN adduser -D -s /bin/sh -u 1001 appuser
WORKDIR /app
COPY --from=builder /app/backend-api .
COPY --from=builder /app/internal/database/migrations ./internal/database/migrations
RUN chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1
CMD ["./backend-api"]
```

### Container Build Commands

#### Local Container Builds

```bash
# Build with Docker
docker build -t cls-backend:dev .

# Build with Podman (recommended)
podman build -t cls-backend:dev .

# Build for specific platform (required for GKE)
podman build --platform linux/amd64 -t cls-backend:dev .
```

#### Production Container Builds

```bash
# Build with build arguments
BUILD_TAG="v1.0.0"
GIT_COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

podman build \
  --platform linux/amd64 \
  --build-arg VERSION=${BUILD_TAG} \
  --build-arg GIT_COMMIT=${GIT_COMMIT} \
  --build-arg BUILD_TIME=${BUILD_TIME} \
  -t gcr.io/your-project/cls-backend:${BUILD_TAG} \
  .
```

### Google Container Registry (GCR)

#### Authentication

```bash
# Authenticate podman with GCR
gcloud auth print-access-token | \
  podman login -u oauth2accesstoken --password-stdin gcr.io
```

#### Push to Registry

```bash
# Push to GCR
REGISTRY_AUTH_FILE=/path/to/auth.json \
  podman push gcr.io/your-project/cls-backend:${BUILD_TAG}

# Verify push
gcloud container images list --repository=gcr.io/your-project
```

## Build Optimization

### Build Performance

#### Dependency Caching

```dockerfile
# Copy go.mod first for better layer caching
COPY go.mod go.sum ./
RUN go mod download
COPY . .
```

#### Multi-stage Benefits

- **Builder Stage**: ~1.2GB (golang:1.21-alpine with build tools)
- **Runtime Stage**: ~20MB (alpine:3.18 + binary + migrations)
- **Final Image**: Optimized for fast deployment and low resource usage

### Build Arguments

```bash
# Common build arguments
--build-arg VERSION=v1.0.0          # Version tag
--build-arg GIT_COMMIT=$(git rev-parse --short HEAD)  # Git commit
--build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")  # Build timestamp
```

These are embedded in the binary and available via `/api/v1/info` endpoint.

## Automated CI/CD

### GitHub Actions Example

```yaml
name: Build and Deploy

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

env:
  PROJECT_ID: your-gcp-project
  IMAGE_NAME: cls-backend

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:13
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: cls_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21

    - name: Run tests
      env:
        DATABASE_URL: postgres://postgres:postgres@localhost:5432/cls_test
      run: |
        go test -v ./internal/...

  build:
    needs: test
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'

    steps:
    - uses: actions/checkout@v3

    - name: Set up Google Cloud SDK
      uses: google-github-actions/setup-gcloud@v0
      with:
        service_account_key: ${{ secrets.GCP_SA_KEY }}
        project_id: ${{ env.PROJECT_ID }}

    - name: Configure Docker
      run: gcloud auth configure-docker

    - name: Build and push
      env:
        IMAGE_TAG: ${{ github.sha }}
      run: |
        docker build \
          --build-arg VERSION=${GITHUB_REF#refs/tags/} \
          --build-arg GIT_COMMIT=${GITHUB_SHA} \
          --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
          -t gcr.io/${PROJECT_ID}/${IMAGE_NAME}:${IMAGE_TAG} \
          .
        docker push gcr.io/${PROJECT_ID}/${IMAGE_NAME}:${IMAGE_TAG}
```

## Makefile Targets

### Available Targets

```makefile
# Build targets
build:           # Build local binary
build-container: # Build container image
build-push:      # Build and push to registry

# Test targets
test:            # Run all tests
test-unit:       # Run unit tests only
test-integration:# Run integration tests
test-coverage:   # Run tests with coverage

# Development targets
run:             # Run local development server
dev:             # Run with hot reload
clean:           # Clean build artifacts

# Deployment targets
deploy:          # Deploy to Kubernetes
deploy-dev:      # Deploy to development environment
```

### Example Makefile

```makefile
# Variables
PROJECT_ID ?= your-gcp-project
IMAGE_NAME = cls-backend
BUILD_TAG ?= latest
GIT_COMMIT = $(shell git rev-parse --short HEAD)
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build binary
build:
	go build -o bin/backend-api ./cmd/backend-api

# Build container
build-container:
	podman build \
		--platform linux/amd64 \
		--build-arg VERSION=$(BUILD_TAG) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t gcr.io/$(PROJECT_ID)/$(IMAGE_NAME):$(BUILD_TAG) \
		.

# Push to registry
push: build-container
	REGISTRY_AUTH_FILE=~/.config/containers/auth.json \
		podman push gcr.io/$(PROJECT_ID)/$(IMAGE_NAME):$(BUILD_TAG)

# Run tests
test:
	go test -v ./internal/...

test-coverage:
	go test -cover ./internal/...

# Development
run: build
	./bin/backend-api

dev:
	air

clean:
	rm -rf bin/
	go clean

.PHONY: build build-container push test test-coverage run dev clean
```

## Troubleshooting Build Issues

### Common Problems

#### 1. Go Module Issues

**Error**: `go: module not found`

**Solutions**:
```bash
# Update dependencies
go mod download
go mod tidy

# Clear module cache
go clean -modcache

# Verify module
go mod verify
```

#### 2. Container Build Failures

**Error**: Platform compatibility issues

**Solutions**:
```bash
# Always specify platform for GKE
podman build --platform linux/amd64 -t image:tag .

# Check available platforms
podman system info | grep -A 10 "Supported platforms"
```

#### 3. Registry Authentication

**Error**: `unauthorized: authentication failed`

**Solutions**:
```bash
# Re-authenticate with GCR
gcloud auth login
gcloud auth configure-docker

# For podman
gcloud auth print-access-token | \
  podman login -u oauth2accesstoken --password-stdin gcr.io

# Verify authentication
podman login --get-login gcr.io
```

#### 4. Image Size Issues

**Problem**: Large container images

**Solutions**:
```bash
# Use multi-stage builds (already implemented)
# Minimize dependencies in runtime stage
# Use alpine base images

# Check image size
podman images cls-backend

# Analyze layers
podman history cls-backend:latest
```

### Build Verification

#### Verify Local Build

```bash
# Build and test locally
make build
./bin/backend-api --version

# Test binary
export DISABLE_AUTH=true
export DATABASE_URL="postgres://user:pass@localhost:5432/test"
./bin/backend-api &
curl http://localhost:8080/health
```

#### Verify Container Build

```bash
# Build container
make build-container

# Test container
podman run -p 8080:8080 \
  -e DISABLE_AUTH=true \
  -e DATABASE_URL="postgres://host.docker.internal:5432/test" \
  cls-backend:latest &

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/info
```

## Image Tagging Strategy

### Development Tags

```bash
# Feature branches
simplified-YYYYMMDD-HHMMSS    # e.g., simplified-20251017-180000
test-FEATURE-YYYYMMDD         # e.g., test-auth-20251017
dev-COMMIT                    # e.g., dev-a1b2c3d
```

### Production Tags

```bash
# Semantic versioning
v1.0.0, v1.0.1, v1.1.0       # Release versions
latest                        # Latest stable release
stable                        # Current stable version
```

### Architecture-Specific Tags

```bash
# Architecture indicators
simplified-architecture       # Single-tenant simplified
external-auth-ready          # External authorization ready
fan-out-complete            # Controller-agnostic pub/sub
```

## Deployment Integration

### Kubernetes Deployment

After successful build, update deployment:

```bash
# Update image in deployment
kubectl set image deployment/cls-backend \
  cls-backend=gcr.io/project/cls-backend:${BUILD_TAG} \
  -n cls-system

# Monitor rollout
kubectl rollout status deployment/cls-backend -n cls-system

# Verify deployment
kubectl get pods -n cls-system -l app=cls-backend
```

### Environment-Specific Builds

```bash
# Development environment
BUILD_TAG="dev-$(date +%Y%m%d-%H%M%S)"
make build-container push BUILD_TAG=${BUILD_TAG}

# Staging environment
BUILD_TAG="staging-$(git rev-parse --short HEAD)"
make build-container push BUILD_TAG=${BUILD_TAG}

# Production environment
BUILD_TAG="v$(cat VERSION)"
make build-container push BUILD_TAG=${BUILD_TAG}
```

## Performance Monitoring

### Build Metrics

Track build performance:
- **Build time**: Monitor container build duration
- **Image size**: Track final image size
- **Cache hit rate**: Monitor layer cache effectiveness
- **Test coverage**: Maintain high test coverage

### Runtime Metrics

After deployment, monitor:
- **Startup time**: Container startup performance
- **Memory usage**: Runtime memory consumption
- **Response time**: API response performance
- **Error rates**: Application error rates

This build process ensures reliable, reproducible, and optimized deployments of CLS Backend across all environments.