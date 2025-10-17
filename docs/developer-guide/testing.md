# Testing Guide

This guide covers the testing strategy, running tests, and best practices for CLS Backend development.

## Testing Strategy

CLS Backend uses a comprehensive testing approach with multiple levels:

1. **Unit Tests** - Test individual functions and components in isolation
2. **Integration Tests** - Test component interactions with real dependencies
3. **API Tests** - Test HTTP endpoints end-to-end
4. **Performance Tests** - Validate performance characteristics

## Running Tests

### Quick Test Commands

```bash
# Run all tests
make test

# Run only unit tests (fast)
make test-unit

# Run integration tests (requires database)
make test-integration

# Run tests with coverage
make test-coverage

# Run tests with verbose output
go test -v ./internal/...
```

### Test Environment Setup

#### 1. Test Database

Tests require a PostgreSQL test database:

```bash
# Create test database
createdb -h localhost -U cls_user cls_test

# Or using Docker
docker run --name cls-test-postgres \
  -e POSTGRES_USER=cls_user \
  -e POSTGRES_PASSWORD=cls_password \
  -e POSTGRES_DB=cls_test \
  -p 5433:5432 \
  -d postgres:13
```

#### 2. Environment Variables

```bash
# Set test environment variables
export TEST_DATABASE_URL="postgres://cls_user:cls_password@localhost:5433/cls_test"
export DISABLE_AUTH=true
export LOG_LEVEL=error  # Reduce log noise during tests
```

#### 3. Google Cloud Emulator (Optional)

For Pub/Sub integration tests:

```bash
# Install emulator
gcloud components install pubsub-emulator

# Start emulator
gcloud beta emulators pubsub start --project=test-project --host-port=localhost:8085

# Set environment
export PUBSUB_EMULATOR_HOST=localhost:8085
export GOOGLE_CLOUD_PROJECT=test-project
```

## Test Structure

### Test Organization

```
internal/
├── api/
│   ├── handlers/
│   │   ├── clusters_test.go      # API handler tests
│   │   └── health_test.go
│   └── middleware/
│       └── auth_test.go          # Middleware tests
├── database/
│   ├── clusters_test.go          # Repository tests
│   └── status_aggregator_test.go # Status logic tests
├── models/
│   └── cluster_test.go           # Model validation tests
└── pubsub/
    └── publisher_test.go         # Event publishing tests
```

### Test Naming Conventions

```go
// Function: CreateCluster
func TestCreateCluster(t *testing.T) { }

// Method: repo.GetCluster
func TestRepository_GetCluster(t *testing.T) { }

// Multiple test cases
func TestClusterValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   Cluster
        wantErr bool
    }{
        {"valid cluster", validCluster, false},
        {"missing name", clusterWithoutName, true},
    }
    // ... test implementation
}
```

## Unit Testing

### Database Repository Tests

```go
// internal/database/clusters_test.go
package database

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRepository_CreateCluster(t *testing.T) {
    repo := setupTestRepository(t)
    defer cleanupTestRepository(t, repo)

    cluster := &models.Cluster{
        Name: "test-cluster",
        Spec: map[string]interface{}{
            "platform": map[string]interface{}{
                "type": "gcp",
            },
        },
        CreatedBy: "test@example.com",
    }

    // Test creation
    created, err := repo.CreateCluster(context.Background(), cluster)
    require.NoError(t, err)
    assert.NotEmpty(t, created.ID)
    assert.Equal(t, int64(1), created.Generation)

    // Test retrieval
    retrieved, err := repo.GetCluster(context.Background(), created.ID, "test@example.com")
    require.NoError(t, err)
    assert.Equal(t, created.ID, retrieved.ID)
    assert.Equal(t, created.Name, retrieved.Name)
}

func setupTestRepository(t *testing.T) *Repository {
    db, err := sql.Open("postgres", getTestDatabaseURL())
    require.NoError(t, err)

    // Apply migrations
    err = runTestMigrations(db)
    require.NoError(t, err)

    return NewRepository(db)
}

func cleanupTestRepository(t *testing.T, repo *Repository) {
    // Clean up test data
    _, err := repo.db.Exec("TRUNCATE TABLE clusters CASCADE")
    require.NoError(t, err)
}
```

### API Handler Tests

```go
// internal/api/handlers/clusters_test.go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestClusterHandler_CreateCluster(t *testing.T) {
    // Setup
    gin.SetMode(gin.TestMode)
    mockRepo := &MockRepository{}
    handler := NewClusterHandler(mockRepo)

    // Mock expectations
    expectedCluster := &models.Cluster{
        ID:        "test-id",
        Name:      "test-cluster",
        Generation: 1,
    }
    mockRepo.On("CreateCluster", mock.Anything, mock.Anything).Return(expectedCluster, nil)

    // Request body
    requestBody := map[string]interface{}{
        "name": "test-cluster",
        "spec": map[string]interface{}{
            "platform": map[string]interface{}{
                "type": "gcp",
            },
        },
    }

    body, _ := json.Marshal(requestBody)
    req := httptest.NewRequest("POST", "/api/v1/clusters", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-User-Email", "test@example.com")

    w := httptest.NewRecorder()
    router := gin.New()
    router.POST("/api/v1/clusters", handler.CreateCluster)
    router.ServeHTTP(w, req)

    // Assertions
    assert.Equal(t, http.StatusCreated, w.Code)

    var response models.Cluster
    err := json.Unmarshal(w.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Equal(t, expectedCluster.ID, response.ID)
    assert.Equal(t, expectedCluster.Name, response.Name)

    mockRepo.AssertExpectations(t)
}

// Mock repository for testing
type MockRepository struct {
    mock.Mock
}

func (m *MockRepository) CreateCluster(ctx context.Context, cluster *models.Cluster) (*models.Cluster, error) {
    args := m.Called(ctx, cluster)
    return args.Get(0).(*models.Cluster), args.Error(1)
}

func (m *MockRepository) GetCluster(ctx context.Context, id, userEmail string) (*models.Cluster, error) {
    args := m.Called(ctx, id, userEmail)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*models.Cluster), args.Error(1)
}
```

### Model Validation Tests

```go
// internal/models/cluster_test.go
package models

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCluster_Validate(t *testing.T) {
    tests := []struct {
        name    string
        cluster Cluster
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid cluster",
            cluster: Cluster{
                Name: "valid-cluster",
                Spec: map[string]interface{}{
                    "platform": map[string]interface{}{
                        "type": "gcp",
                    },
                },
                CreatedBy: "user@example.com",
            },
            wantErr: false,
        },
        {
            name: "missing name",
            cluster: Cluster{
                Spec:      map[string]interface{}{},
                CreatedBy: "user@example.com",
            },
            wantErr: true,
            errMsg:  "name is required",
        },
        {
            name: "invalid name format",
            cluster: Cluster{
                Name:      "Invalid_Name!",
                Spec:      map[string]interface{}{},
                CreatedBy: "user@example.com",
            },
            wantErr: true,
            errMsg:  "name must contain only lowercase letters, numbers, and hyphens",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.cluster.Validate()
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Integration Testing

### Database Integration Tests

```go
// internal/database/integration_test.go
package database

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestClusterLifecycle_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    repo := setupIntegrationTest(t)
    defer cleanupIntegrationTest(t, repo)

    ctx := context.Background()
    userEmail := "integration@test.com"

    // Create cluster
    cluster := &models.Cluster{
        Name: "integration-test-cluster",
        Spec: map[string]interface{}{
            "platform": map[string]interface{}{
                "type": "gcp",
                "gcp": map[string]interface{}{
                    "projectID": "test-project",
                    "region":    "us-central1",
                },
            },
        },
        CreatedBy: userEmail,
    }

    created, err := repo.CreateCluster(ctx, cluster)
    require.NoError(t, err)
    assert.NotEmpty(t, created.ID)

    // Update cluster
    created.Spec["platform"].(map[string]interface{})["gcp"].(map[string]interface{})["region"] = "us-west1"
    updated, err := repo.UpdateCluster(ctx, created)
    require.NoError(t, err)
    assert.Equal(t, int64(2), updated.Generation)

    // Get cluster
    retrieved, err := repo.GetCluster(ctx, created.ID, userEmail)
    require.NoError(t, err)
    assert.Equal(t, updated.Generation, retrieved.Generation)

    // List clusters
    clusters, total, err := repo.ListClusters(ctx, userEmail, 50, 0)
    require.NoError(t, err)
    assert.GreaterOrEqual(t, total, int64(1))
    assert.Len(t, clusters, 1)

    // Delete cluster
    err = repo.DeleteCluster(ctx, created.ID, userEmail)
    require.NoError(t, err)

    // Verify deletion
    _, err = repo.GetCluster(ctx, created.ID, userEmail)
    assert.Error(t, err)
}
```

### API Integration Tests

```go
// test/integration/api_test.go
package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAPI_ClusterCRUD_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    client := setupIntegrationClient(t)
    userEmail := "integration@test.com"

    // Create cluster
    createReq := map[string]interface{}{
        "name": "api-integration-test",
        "spec": map[string]interface{}{
            "platform": map[string]interface{}{
                "type": "gcp",
            },
        },
    }

    cluster := client.CreateCluster(t, createReq, userEmail)
    assert.NotEmpty(t, cluster.ID)
    assert.Equal(t, int64(1), cluster.Generation)

    // Get cluster
    retrieved := client.GetCluster(t, cluster.ID, userEmail)
    assert.Equal(t, cluster.ID, retrieved.ID)

    // Update cluster
    updateReq := map[string]interface{}{
        "spec": map[string]interface{}{
            "platform": map[string]interface{}{
                "type": "aws",
            },
        },
    }

    updated := client.UpdateCluster(t, cluster.ID, updateReq, userEmail)
    assert.Equal(t, int64(2), updated.Generation)

    // List clusters
    clusters := client.ListClusters(t, userEmail)
    assert.GreaterOrEqual(t, len(clusters), 1)

    // Delete cluster
    client.DeleteCluster(t, cluster.ID, userEmail)

    // Verify deletion returns 404
    client.GetClusterExpecting404(t, cluster.ID, userEmail)
}

type IntegrationClient struct {
    baseURL string
    client  *http.Client
}

func (c *IntegrationClient) CreateCluster(t *testing.T, req map[string]interface{}, userEmail string) *models.Cluster {
    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", c.baseURL+"/api/v1/clusters", bytes.NewBuffer(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-User-Email", userEmail)

    resp, err := c.client.Do(httpReq)
    require.NoError(t, err)
    defer resp.Body.Close()

    require.Equal(t, http.StatusCreated, resp.StatusCode)

    var cluster models.Cluster
    err = json.NewDecoder(resp.Body).Decode(&cluster)
    require.NoError(t, err)

    return &cluster
}
```

## Test Data Management

### Test Fixtures

```go
// internal/testutil/fixtures.go
package testutil

import "github.com/your-org/cls-backend/internal/models"

func CreateTestCluster(name string) *models.Cluster {
    return &models.Cluster{
        Name: name,
        Spec: map[string]interface{}{
            "platform": map[string]interface{}{
                "type": "gcp",
                "gcp": map[string]interface{}{
                    "projectID": "test-project",
                    "region":    "us-central1",
                },
            },
        },
        CreatedBy: "test@example.com",
    }
}

func CreateTestClusterWithPlatform(name, platform string) *models.Cluster {
    cluster := CreateTestCluster(name)
    cluster.Spec["platform"].(map[string]interface{})["type"] = platform
    return cluster
}
```

### Database Test Helpers

```go
// internal/testutil/database.go
package testutil

import (
    "database/sql"
    "testing"
    "github.com/stretchr/testify/require"
)

func SetupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("postgres", getTestDatabaseURL())
    require.NoError(t, err)

    // Apply migrations
    err = runMigrations(db)
    require.NoError(t, err)

    return db
}

func CleanupTestDB(t *testing.T, db *sql.DB) {
    // Clean all tables
    tables := []string{"controller_status", "reconciliation_schedule", "clusters"}
    for _, table := range tables {
        _, err := db.Exec("TRUNCATE TABLE " + table + " CASCADE")
        require.NoError(t, err)
    }
}

func SeedTestData(t *testing.T, db *sql.DB) {
    // Insert test data
    testClusters := []string{"test-cluster-1", "test-cluster-2"}
    for _, name := range testClusters {
        _, err := db.Exec(`
            INSERT INTO clusters (name, spec, created_by)
            VALUES ($1, '{"platform":{"type":"gcp"}}', 'test@example.com')
        `, name)
        require.NoError(t, err)
    }
}
```

## Performance Testing

### Benchmark Tests

```go
// internal/database/clusters_bench_test.go
package database

import (
    "context"
    "testing"
)

func BenchmarkRepository_GetCluster(b *testing.B) {
    repo := setupBenchmarkRepository(b)
    defer cleanupBenchmarkRepository(b, repo)

    // Setup test data
    cluster := createBenchmarkCluster(b, repo)
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := repo.GetCluster(ctx, cluster.ID, cluster.CreatedBy)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkRepository_ListClusters(b *testing.B) {
    repo := setupBenchmarkRepository(b)
    defer cleanupBenchmarkRepository(b, repo)

    // Create test clusters
    for i := 0; i < 100; i++ {
        createBenchmarkCluster(b, repo)
    }

    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _, err := repo.ListClusters(ctx, "test@example.com", 50, 0)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Load Testing

```bash
#!/bin/bash
# scripts/load-test.sh

# Simple load test using curl
BASE_URL="http://localhost:8080"
USER_EMAIL="loadtest@example.com"
CONCURRENT=10
REQUESTS=100

echo "Running load test: $CONCURRENT concurrent users, $REQUESTS requests each"

# Health check load test
echo "Testing /health endpoint..."
for i in $(seq 1 $CONCURRENT); do
  (
    for j in $(seq 1 $REQUESTS); do
      curl -s "$BASE_URL/health" > /dev/null
    done
  ) &
done
wait

# API load test
echo "Testing /api/v1/clusters endpoint..."
for i in $(seq 1 $CONCURRENT); do
  (
    for j in $(seq 1 $REQUESTS); do
      curl -s -H "X-User-Email: $USER_EMAIL" \
        "$BASE_URL/api/v1/clusters" > /dev/null
    done
  ) &
done
wait

echo "Load test completed"
```

## Test Best Practices

### 1. Test Organization
- Keep tests close to the code they test
- Use descriptive test names that explain the scenario
- Group related tests using subtests or test suites
- Use table-driven tests for multiple scenarios

### 2. Test Independence
- Each test should be independent and isolated
- Clean up test data after each test
- Don't rely on test execution order
- Use fresh test data for each test

### 3. Mocking and Stubbing
- Mock external dependencies (databases, APIs, Pub/Sub)
- Use interfaces to make components testable
- Don't mock types you don't own
- Keep mocks simple and focused

### 4. Test Data
- Use minimal test data that covers the scenario
- Create factories/builders for complex test objects
- Use realistic but non-sensitive test data
- Version control test data when appropriate

### 5. Performance
- Use `testing.Short()` to skip slow tests in quick runs
- Profile slow tests to identify bottlenecks
- Use benchmarks for performance-critical code
- Consider parallel test execution for independent tests

## Continuous Integration

### GitHub Actions Test Workflow

```yaml
# .github/workflows/test.yml
name: Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

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
        ports:
          - 5432:5432

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21

    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-

    - name: Run tests
      env:
        TEST_DATABASE_URL: postgres://postgres:postgres@localhost:5432/cls_test
        DISABLE_AUTH: true
      run: |
        go test -v -race -coverprofile=coverage.out ./internal/...

    - name: Upload coverage to Codecov
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
```

## Related Documentation

- **[Local Setup](local-setup.md)** - Development environment setup
- **[API Development](api-development.md)** - Adding new features
- **[Architecture](architecture.md)** - System design overview
- **[Build Process](build-process.md)** - Building and deployment