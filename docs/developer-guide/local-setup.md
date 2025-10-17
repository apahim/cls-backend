# Local Development Setup

This guide will help you set up a complete local development environment for CLS Backend.

## Prerequisites

### Required Software

- **Go 1.21+** - Primary programming language
- **PostgreSQL 13+** - Database server
- **Docker/Podman** - Container runtime for testing and deployment
- **Git** - Version control
- **Make** - Build automation

### Optional Tools

- **Google Cloud SDK** - For GCP integration testing
- **kubectl** - Kubernetes command-line tool
- **jq** - JSON processor for API testing

## Installation Steps

### 1. Clone Repository

```bash
git clone https://github.com/your-org/cls-backend.git
cd cls-backend
```

### 2. Install Go Dependencies

```bash
# Download dependencies
go mod download

# Verify dependencies
go mod tidy
```

### 3. Setup PostgreSQL Database

#### Option A: Docker PostgreSQL (Recommended)

```bash
# Start PostgreSQL in Docker
docker run --name cls-postgres \
  -e POSTGRES_USER=cls_user \
  -e POSTGRES_PASSWORD=cls_password \
  -e POSTGRES_DB=cls_backend \
  -p 5432:5432 \
  -d postgres:13

# Verify connection
psql "postgres://cls_user:cls_password@localhost:5432/cls_backend" -c "SELECT version();"
```

#### Option B: Local PostgreSQL

```bash
# macOS with Homebrew
brew install postgresql@13
brew services start postgresql@13

# Ubuntu/Debian
sudo apt update
sudo apt install postgresql-13 postgresql-contrib

# Create database and user
sudo -u postgres createuser cls_user
sudo -u postgres createdb cls_backend
sudo -u postgres psql -c "ALTER USER cls_user PASSWORD 'cls_password';"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE cls_backend TO cls_user;"
```

### 4. Environment Configuration

Create environment variables for local development:

```bash
# Create .env file
cat > .env << 'EOF'
# Database
DATABASE_URL=postgres://cls_user:cls_password@localhost:5432/cls_backend

# Google Cloud (optional for local development)
GOOGLE_CLOUD_PROJECT=your-test-project

# Development settings
DISABLE_AUTH=true
LOG_LEVEL=debug
PORT=8080

# Test database (for running tests)
TEST_DATABASE_URL=postgres://cls_user:cls_password@localhost:5432/cls_test
EOF

# Load environment variables
source .env
```

### 5. Database Setup

#### Create Test Database

```bash
# Create test database
createdb -h localhost -U cls_user cls_test

# Or with psql
psql "postgres://cls_user:cls_password@localhost:5432/postgres" -c "CREATE DATABASE cls_test;"
```

#### Apply Migrations

CLS Backend includes database migrations in the container. For local development:

```bash
# Migrations are applied automatically when the service starts
# Or manually run migrations using psql:
psql "$DATABASE_URL" -f internal/database/migrations/001_complete_schema.sql
```

### 6. Build and Test

```bash
# Build the application
make build

# Run unit tests
make test-unit

# Run integration tests (requires database)
make test-integration

# Run all tests
make test
```

### 7. Start Development Server

```bash
# Option A: Using built binary
./bin/backend-api

# Option B: Using go run
go run ./cmd/backend-api

# Option C: Using Make
make run
```

You should see output like:
```
INFO Starting CLS Backend server on :8080
INFO Database connection established
INFO Pub/Sub client initialized (development mode)
```

## Development Workflow

### Testing Your Setup

#### Health Check

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "pubsub": "degraded"
  }
}
```

Note: Pub/Sub will show "degraded" in local development without GCP credentials.

#### Basic API Test

```bash
# Create a cluster
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "X-User-Email: dev@example.com" \
  -d '{
    "name": "test-cluster",
    "spec": {
      "platform": {
        "type": "gcp"
      }
    }
  }'

# List clusters
curl -H "X-User-Email: dev@example.com" \
  http://localhost:8080/api/v1/clusters
```

### Code Changes and Testing

#### Automatic Restart During Development

Use `air` for automatic restarts on code changes:

```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run with auto-restart
air
```

Create `.air.toml` configuration:
```toml
root = "."
cmd = "go run ./cmd/backend-api"
bin = "./bin/backend-api"
include_ext = ["go", "tpl", "tmpl", "html"]
exclude_dir = ["tmp", "vendor", "node_modules"]
```

#### Running Tests During Development

```bash
# Run tests with verbose output
go test -v ./internal/...

# Run specific test
go test -v ./internal/api -run TestClusterHandlers

# Run tests with coverage
go test -cover ./internal/...

# Watch tests (requires gotestsum)
gotestsum --watch
```

## Google Cloud Integration (Optional)

For full feature testing with Google Cloud Pub/Sub:

### 1. Setup GCP Credentials

```bash
# Install Google Cloud SDK
# macOS: brew install google-cloud-sdk
# Ubuntu: snap install google-cloud-sdk --classic

# Authenticate
gcloud auth login
gcloud auth application-default login

# Set project
gcloud config set project your-test-project
```

### 2. Enable Required APIs

```bash
gcloud services enable pubsub.googleapis.com
```

### 3. Create Pub/Sub Resources

```bash
# Create topic
gcloud pubsub topics create cluster-events

# Verify
gcloud pubsub topics list
```

### 4. Update Environment

```bash
# Add to .env
echo "GOOGLE_CLOUD_PROJECT=$(gcloud config get-value project)" >> .env
source .env
```

## Container Development

### Building Containers Locally

```bash
# Build with Docker
docker build -t cls-backend:dev .

# Build with Podman
podman build -t cls-backend:dev .

# Build for specific platform (for GKE)
podman build --platform linux/amd64 -t cls-backend:dev .
```

### Running in Container

```bash
# Run locally built container
docker run -p 8080:8080 \
  -e DATABASE_URL="$DATABASE_URL" \
  -e DISABLE_AUTH=true \
  cls-backend:dev

# With host networking (easier for database access)
docker run --network host \
  -e DATABASE_URL="$DATABASE_URL" \
  -e DISABLE_AUTH=true \
  cls-backend:dev
```

## Troubleshooting

### Common Issues

#### Database Connection Failed

**Error**: `Failed to connect to database: connection refused`

**Solutions**:
1. Verify PostgreSQL is running: `pg_isready -h localhost -p 5432`
2. Check DATABASE_URL format
3. Verify user permissions: `psql "$DATABASE_URL" -c "SELECT version();"`

#### Build Failures

**Error**: `go: module not found` or compilation errors

**Solutions**:
1. Update dependencies: `go mod download && go mod tidy`
2. Clean module cache: `go clean -modcache`
3. Verify Go version: `go version` (should be 1.21+)

#### Port Already in Use

**Error**: `bind: address already in use`

**Solutions**:
1. Check what's using port 8080: `lsof -i :8080`
2. Kill the process: `kill $(lsof -t -i:8080)`
3. Use different port: `export PORT=8081`

#### Tests Failing

**Error**: Test database connection issues

**Solutions**:
1. Create test database: `createdb -h localhost -U cls_user cls_test`
2. Set TEST_DATABASE_URL in environment
3. Run tests with proper environment: `source .env && make test-unit`

### Development Tips

#### Database Reset

```bash
# Drop and recreate database
dropdb -h localhost -U cls_user cls_backend
createdb -h localhost -U cls_user cls_backend

# Restart application to run migrations
```

#### Debugging

```bash
# Enable debug logging
export LOG_LEVEL=debug

# Use delve debugger
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/backend-api
```

#### API Testing

```bash
# Test with curl and save responses
curl -v -H "X-User-Email: dev@example.com" \
  http://localhost:8080/api/v1/clusters > clusters.json

# Use httpie for easier API testing
pip install httpie
http localhost:8080/api/v1/clusters X-User-Email:dev@example.com
```

## IDE Setup

### VS Code

Recommended extensions:
- Go (official Go extension)
- REST Client (for API testing)
- PostgreSQL (database management)

Create `.vscode/settings.json`:
```json
{
  "go.useLanguageServer": true,
  "go.formatTool": "goimports",
  "go.lintTool": "golangci-lint",
  "go.testFlags": ["-v"],
  "go.buildFlags": ["-v"]
}
```

### GoLand/IntelliJ

- Configure Go SDK to point to your Go installation
- Set GOPATH and GOROOT correctly
- Enable Go modules support
- Configure database connection for SQL editing

## Next Steps

Once your local environment is working:

1. **Read Architecture Guide**: Understand the system design
2. **Review Code Style**: Follow our coding conventions
3. **Practice with Tests**: Run and understand the test suite
4. **Make Changes**: Start with small improvements or bug fixes
5. **Submit PR**: Follow our contribution guidelines

For more detailed development guidance, see:
- [Architecture Overview](architecture.md)
- [Testing Guide](testing.md)
- [API Development](api-development.md)