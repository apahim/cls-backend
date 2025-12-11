# Variables
PROJECT_ID ?= your-project-id
IMAGE_REGISTRY ?= gcr.io
IMAGE_NAME ?= cls-backend
IMAGE_TAG ?= latest
FULL_IMAGE_NAME = $(IMAGE_REGISTRY)/$(PROJECT_ID)/$(IMAGE_NAME):$(IMAGE_TAG)

# Go variables
GO_VERSION = 1.23
MAIN_PACKAGE = ./cmd/backend-api
BINARY_NAME = backend-api

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_TIME = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# LDFLAGS for version info
LDFLAGS = -ldflags="-w -s -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Default target
.PHONY: all
all: test build

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build               - Build the binary"
	@echo "  test                - Run comprehensive test suite"
	@echo "  test-unit           - Run unit tests only (no external dependencies)"
	@echo "  test-integration    - Run integration tests (requires database)"
	@echo "  test-coverage       - Generate test coverage report"
	@echo "  test-package        - Run tests for specific package (PKG=package)"
	@echo "  test-fast           - Run tests without race detector"
	@echo "  test-no-db          - Run tests without database dependencies"
	@echo "  lint                - Run linters"
	@echo "  fmt                 - Format code"
	@echo "  clean               - Clean build artifacts"
	@echo "  docker-build        - Build Docker image"
	@echo "  cloud-build         - Build image using Google Cloud Build"
	@echo "  docker-push         - Push Docker image"
	@echo "  deploy-local        - Deploy to local kind cluster"
	@echo "  deploy-k8s          - Deploy to Kubernetes"
	@echo "  migrate-up          - Run database migrations"
	@echo "  migrate-down        - Rollback database migrations"

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PACKAGE)

# Build for multiple platforms
.PHONY: build-all
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

# Run all tests
.PHONY: test
test:
	@echo "Running comprehensive test suite..."
	@go test -v -race ./...

# Run unit tests only (no external dependencies)
.PHONY: test-unit
test-unit:
	@echo "Running unit tests (pure business logic)..."
	@echo "Testing models package..."
	@cd internal/models && go test -v .
	@echo "Testing utils package..."
	@cd internal/utils && go test -v .
	@echo "Testing config package..."
	@cd internal/config && go test -v .

# Run integration tests (requires external dependencies)
.PHONY: test-integration
test-integration:
	@echo "Running integration tests (requires database)..."
	@cd integration_tests && go test -v -tags=integration .

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@echo "Generating coverage report for working packages..."
	@mkdir -p coverage
	@rm -f coverage/*.out coverage/*.html
	@go test -coverprofile=coverage/models.out ./internal/models
	@go test -coverprofile=coverage/config.out ./internal/config
	@echo "mode: atomic" > coverage/coverage.out
	@grep -h -v "^mode:" coverage/models.out coverage/config.out >> coverage/coverage.out || true
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@go tool cover -func=coverage/coverage.out | grep "total:" || echo "Coverage report generated"
	@echo "Coverage report generated: coverage/coverage.html"

# Run tests for specific package
.PHONY: test-package
test-package:
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make test-package PKG=package_name"; \
		echo "Example: make test-package PKG=internal/database"; \
		exit 1; \
	fi
	@echo "Running tests for package: $(PKG)"
	@go test -v -race $(PKG)

# Run tests without race detector (faster)
.PHONY: test-fast
test-fast:
	@echo "Running tests without race detector..."
	@go test -v ./...

# Run tests without database (CI environments)
.PHONY: test-no-db
test-no-db:
	@echo "Running tests without database dependencies..."
	@SKIP_DB_TESTS=true go test -v ./...

# Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "Running benchmarks..."
	go test -v -bench=. -benchmem ./...

# Lint code
.PHONY: lint
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run ./...; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports not installed. Installing..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
		goimports -w .; \
	fi

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Generate code
.PHONY: generate
generate:
	@echo "Generating code..."
	go generate ./...

# Docker targets
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	podman build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(FULL_IMAGE_NAME) .

.PHONY: podman-push
docker-push: docker-build
	@echo "Pushing Docker image..."
	podman push $(FULL_IMAGE_NAME)

.PHONY: cloud-build
cloud-build:
	@echo "Building image using Google Cloud Build..."
	@if [ "$(PROJECT_ID)" = "your-project-id" ]; then \
		echo "Error: Please set PROJECT_ID environment variable or override with make cloud-build PROJECT_ID=your-project-id"; \
		exit 1; \
	fi
	gcloud builds submit --tag $(FULL_IMAGE_NAME) . --project=$(PROJECT_ID)

.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 -p 8081:8081 \
		-e DATABASE_URL="postgres://cls_user:cls_password@host.docker.internal:5432/cls_backend?sslmode=disable" \
		-e GOOGLE_CLOUD_PROJECT="local-dev" \
		-e PUBSUB_EMULATOR_HOST="host.docker.internal:8085" \
		-e LOG_LEVEL="debug" \
		-e DISABLE_AUTH="true" \
		$(FULL_IMAGE_NAME)

# Local development
.PHONY: dev
dev:
	@echo "Starting development server..."
	air -c .air.toml

.PHONY: deploy-local
deploy-local: docker-build-local setup-ingress
	@echo "Deploying locally to kind cluster..."
	@echo "Saving and loading image into kind..."
	/opt/podman/bin/podman save cls-backend:dev -o /tmp/cls-backend-dev.tar
	kind load image-archive /tmp/cls-backend-dev.tar --name cls
	rm -f /tmp/cls-backend-dev.tar
	@echo "Applying Kubernetes manifests..."
	kubectl apply -f deploy/local/
	kubectl apply -f deploy/local-ingress/
	@echo "Waiting for deployments to be ready..."
	kubectl wait --for=condition=available --timeout=300s deployment/postgres -n cls-local
	kubectl wait --for=condition=available --timeout=300s deployment/pubsub-emulator -n cls-local
	@echo "Initializing Pub/Sub topics and subscriptions..."
	kubectl wait --for=condition=complete --timeout=120s job/pubsub-init -n cls-local
	kubectl wait --for=condition=available --timeout=300s deployment/cls-backend -n cls-local
	@echo ""
	@echo "✅ Services deployed successfully!"
	@echo ""
	@echo "🌐 Access endpoints via Ingress (NodePort 30080):"
	@echo "  API:     http://localhost:30080/api/v1/clusters"
	@echo "  Health:  http://localhost:30080/health"
	@echo "  Swagger: http://localhost:30080/swagger/index.html (if available)"
	@echo ""
	@echo "🔧 Alternative access methods:"
	@echo "  Custom Host: http://cls-backend.local (add to /etc/hosts: 127.0.0.1 cls-backend.local)"
	@echo ""
	@echo "To check status: kubectl get pods -n cls-local"
	@echo "To view logs: kubectl logs -f deployment/cls-backend -n cls-local"
	@echo "To check ingress: kubectl get ingress -n cls-local"

.PHONY: deploy-local-down
deploy-local-down:
	@echo "Stopping local deployment..."
	kubectl delete -f deploy/local/ --ignore-not-found=true
	@echo "Local deployment stopped"

.PHONY: docker-build-local
docker-build-local:
	@echo "Building Docker image for local development..."
	/opt/podman/bin/podman build \
		--build-arg VERSION=dev \
		--build-arg GIT_COMMIT=local \
		--build-arg BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		-t cls-backend:dev .

.PHONY: setup-ingress
setup-ingress:
	@echo "Setting up ingress controller..."
	@if ! kubectl get namespace ingress-nginx >/dev/null 2>&1; then \
		echo "Installing custom nginx ingress controller..."; \
		kubectl apply -f deploy/local-ingress/nginx-ingress.yaml; \
	else \
		echo "Nginx ingress controller already installed"; \
	fi
	@echo "Waiting for ingress controller to be ready..."
	@kubectl wait --namespace ingress-nginx --for=condition=available deployment/nginx-ingress-controller --timeout=300s || true

# Database migration targets
.PHONY: migrate-up
migrate-up:
	@echo "Running database migrations..."
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "ERROR: DATABASE_URL environment variable is required"; \
		exit 1; \
	fi
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path ./internal/database/migrations -database "$(DATABASE_URL)" up; \
	else \
		echo "migrate tool not installed. Please install: https://github.com/golang-migrate/migrate"; \
		exit 1; \
	fi

.PHONY: migrate-down
migrate-down:
	@echo "Rolling back database migrations..."
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "ERROR: DATABASE_URL environment variable is required"; \
		exit 1; \
	fi
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path ./internal/database/migrations -database "$(DATABASE_URL)" down; \
	else \
		echo "migrate tool not installed. Please install: https://github.com/golang-migrate/migrate"; \
		exit 1; \
	fi

.PHONY: migrate-create
migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Usage: make migrate-create NAME=migration_name"; \
		exit 1; \
	fi
	@if command -v migrate >/dev/null 2>&1; then \
		migrate create -ext sql -dir ./internal/database/migrations -seq $(NAME); \
	else \
		echo "migrate tool not installed. Please install: https://github.com/golang-migrate/migrate"; \
		exit 1; \
	fi

# Kubernetes deployment
.PHONY: deploy-k8s
deploy-k8s:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f deploy/kubernetes/

.PHONY: undeploy-k8s
undeploy-k8s:
	@echo "Removing from Kubernetes..."
	kubectl delete -f deploy/kubernetes/

# Development tools
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/cosmtrek/air@latest
	@echo "Tools installed successfully"

# Version info
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

# Watch and rebuild
.PHONY: watch
watch:
	@echo "Watching for changes..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Installing..."; \
		go install github.com/cosmtrek/air@latest; \
		air; \
	fi
