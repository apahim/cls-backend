package utils

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// TestDBConfig holds test database configuration
type TestDBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// GetTestDBConfig returns test database configuration
func GetTestDBConfig() TestDBConfig {
	return TestDBConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
		User:     getEnvOrDefault("TEST_DB_USER", "cls_test"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", "cls_test"),
		DBName:   getEnvOrDefault("TEST_DB_NAME", "cls_backend_test"),
		SSLMode:  getEnvOrDefault("TEST_DB_SSLMODE", "disable"),
	}
}

// GetTestDBURL returns the test database connection URL
func GetTestDBURL() string {
	cfg := GetTestDBConfig()
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode)
}

// SetupTestDB creates a test database and returns the connection URL
func SetupTestDB(t *testing.T) string {
	cfg := GetTestDBConfig()

	// Generate unique database name for this test
	testDBName := fmt.Sprintf("%s_%s", cfg.DBName, uuid.New().String()[:8])

	// Connect to postgres database to create test database
	adminURL := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.SSLMode)

	db, err := sql.Open("postgres", adminURL)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to test database: %v", err)
	}
	defer db.Close()

	// Create test database
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	if err != nil {
		t.Skipf("Skipping test: cannot create test database: %v", err)
	}

	// Return test database URL
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, testDBName, cfg.SSLMode)

	// Cleanup function
	t.Cleanup(func() {
		// Drop test database
		_, err := db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
		if err != nil {
			t.Logf("Failed to cleanup test database: %v", err)
		}
	})

	return testURL
}

// SkipIfShort skips the test if running in short mode
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

// SkipIfNoTestDB skips the test if test database is not available
func SkipIfNoTestDB(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	cfg := GetTestDBConfig()
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.SSLMode)

	db, err := sql.Open("postgres", testURL)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to test database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test: test database not available: %v", err)
	}
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// AssertError checks if error matches expected condition
func AssertError(t *testing.T, err error, expectError bool, msgAndArgs ...interface{}) {
	t.Helper()
	if expectError && err == nil {
		t.Errorf("Expected error but got none. %v", msgAndArgs)
	}
	if !expectError && err != nil {
		t.Errorf("Expected no error but got: %v. %v", err, msgAndArgs)
	}
}

// AssertEqual checks if two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v. %v", expected, actual, msgAndArgs)
	}
}

// AssertNotEqual checks if two values are not equal
func AssertNotEqual(t *testing.T, notExpected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if notExpected == actual {
		t.Errorf("Expected not %v, but got %v. %v", notExpected, actual, msgAndArgs)
	}
}

// AssertNotNil checks if value is not nil
// Handles the Go interface gotcha where typed nil pointers are not nil interfaces
func AssertNotNil(t *testing.T, value interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if isNil(value) {
		t.Errorf("Expected value to be not nil. %v", msgAndArgs)
	}
}

// AssertNil checks if value is nil
// Handles the Go interface gotcha where typed nil pointers are not nil interfaces
func AssertNil(t *testing.T, value interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if !isNil(value) {
		t.Errorf("Expected value to be nil, got %v. %v", value, msgAndArgs)
	}
}

// isNil checks if a value is nil, handling the interface nil gotcha
// In Go, a typed nil pointer wrapped in an interface is not nil
func isNil(value interface{}) bool {
	if value == nil {
		return true
	}

	// Use reflection to check if the underlying value is nil
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// AssertTrue checks if condition is true
func AssertTrue(t *testing.T, condition bool, msgAndArgs ...interface{}) {
	t.Helper()
	if !condition {
		t.Errorf("Expected condition to be true. %v", msgAndArgs)
	}
}

// AssertFalse checks if condition is false
func AssertFalse(t *testing.T, condition bool, msgAndArgs ...interface{}) {
	t.Helper()
	if condition {
		t.Errorf("Expected condition to be false. %v", msgAndArgs)
	}
}

// AssertContains checks if string contains substring
func AssertContains(t *testing.T, str, substr string, msgAndArgs ...interface{}) {
	t.Helper()
	if !strings.Contains(str, substr) {
		t.Errorf("Expected '%s' to contain '%s'. %v", str, substr, msgAndArgs)
	}
}
