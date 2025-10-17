package config

import (
	"os"
	"testing"

	"github.com/apahim/cls-backend/internal/utils"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "valid configuration",
			envVars: map[string]string{
				"DATABASE_URL":         "postgres://user:pass@localhost:5432/db",
				"GOOGLE_CLOUD_PROJECT": "test-project",
			},
			wantErr: false,
		},
		{
			name: "missing DATABASE_URL",
			envVars: map[string]string{
				"GOOGLE_CLOUD_PROJECT": "test-project",
			},
			wantErr: true,
		},
		{
			name: "missing GOOGLE_CLOUD_PROJECT",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearEnv(t)

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			cfg, err := Load()

			if tt.wantErr {
				utils.AssertError(t, err, true, "Expected error for test case: %s", tt.name)
				// utils.AssertNil(t, cfg, "Config should be nil when error occurs") // TODO: Fix this assertion
			} else {
				utils.AssertError(t, err, false, "Expected no error for test case: %s", tt.name)
				utils.AssertNotNil(t, cfg, "Config should not be nil")

				// Verify required fields are set
				utils.AssertNotEqual(t, "", cfg.Database.URL, "Database URL should be set")
				utils.AssertNotEqual(t, "", cfg.PubSub.ProjectID, "Project ID should be set")
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	// Clear environment and set only required variables
	clearEnv(t)
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")

	cfg, err := Load()
	utils.AssertError(t, err, false, "Should load config with defaults")
	utils.AssertNotNil(t, cfg, "Config should not be nil")

	// Test default values
	utils.AssertEqual(t, 8080, cfg.Server.Port, "Default port should be 8080")
	utils.AssertEqual(t, "development", cfg.Server.Environment, "Default environment should be development")
	utils.AssertEqual(t, 30, cfg.Server.ReadTimeoutSeconds, "Default read timeout")
	utils.AssertEqual(t, 30, cfg.Server.WriteTimeoutSeconds, "Default write timeout")
	utils.AssertEqual(t, 120, cfg.Server.IdleTimeoutSeconds, "Default idle timeout")

	utils.AssertEqual(t, 25, cfg.Database.MaxOpenConns, "Default max open connections")
	utils.AssertEqual(t, 5, cfg.Database.MaxIdleConns, "Default max idle connections")

	utils.AssertEqual(t, "cluster-events", cfg.PubSub.ClusterEventsTopic, "Default cluster events topic")

	utils.AssertEqual(t, "info", cfg.Logging.Level, "Default log level")
	utils.AssertEqual(t, "json", cfg.Logging.Format, "Default log format")
}

func TestServerConfigCustomValues(t *testing.T) {
	clearEnv(t)
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	os.Setenv("PORT", "9090")
	os.Setenv("ENVIRONMENT", "production")
	os.Setenv("SERVER_READ_TIMEOUT_SECONDS", "45")
	os.Setenv("DISABLE_AUTH", "true")
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com,https://app.example.com")

	cfg, err := Load()
	utils.AssertError(t, err, false, "Should load config with custom values")

	utils.AssertEqual(t, 9090, cfg.Server.Port, "Custom port")
	utils.AssertEqual(t, "production", cfg.Server.Environment, "Custom environment")
	utils.AssertEqual(t, 45, cfg.Server.ReadTimeoutSeconds, "Custom read timeout")
	utils.AssertEqual(t, false, cfg.Auth.Enabled, "Auth should be disabled")
	utils.AssertEqual(t, 2, len(cfg.Server.CorsAllowedOrigins), "CORS origins count")
	utils.AssertEqual(t, "https://example.com", cfg.Server.CorsAllowedOrigins[0], "First CORS origin")
	utils.AssertEqual(t, "https://app.example.com", cfg.Server.CorsAllowedOrigins[1], "Second CORS origin")
}

func TestPubSubConfigCustomValues(t *testing.T) {
	clearEnv(t)
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	os.Setenv("GOOGLE_CLOUD_PROJECT", "custom-project")
	os.Setenv("PUBSUB_CLUSTER_EVENTS_TOPIC", "custom-cluster-events")
	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085")
	os.Setenv("PUBSUB_MAX_CONCURRENT_HANDLERS", "20")
	os.Setenv("PUBSUB_MAX_OUTSTANDING_MESSAGES", "200")

	cfg, err := Load()
	utils.AssertError(t, err, false, "Should load config with custom PubSub values")

	utils.AssertEqual(t, "custom-project", cfg.PubSub.ProjectID, "Custom project ID")
	utils.AssertEqual(t, "custom-cluster-events", cfg.PubSub.ClusterEventsTopic, "Custom cluster events topic")
	utils.AssertEqual(t, "localhost:8085", cfg.PubSub.EmulatorHost, "Custom emulator host")
	utils.AssertEqual(t, 20, cfg.PubSub.MaxConcurrentHandlers, "Custom max concurrent handlers")
	utils.AssertEqual(t, 200, cfg.PubSub.MaxOutstandingMessages, "Custom max outstanding messages")
}


func TestGetStringSliceEnv(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue []string
		expected     []string
	}{
		{
			name:         "empty value",
			envValue:     "",
			defaultValue: []string{"default"},
			expected:     []string{"default"},
		},
		{
			name:         "single value",
			envValue:     "single",
			defaultValue: []string{"default"},
			expected:     []string{"single"},
		},
		{
			name:         "multiple values",
			envValue:     "one,two,three",
			defaultValue: []string{"default"},
			expected:     []string{"one", "two", "three"},
		},
		{
			name:         "values with spaces",
			envValue:     " one , two , three ",
			defaultValue: []string{"default"},
			expected:     []string{"one", "two", "three"},
		},
		{
			name:         "empty values in list",
			envValue:     "one,,three",
			defaultValue: []string{"default"},
			expected:     []string{"one", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_STRING_SLICE"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
			} else {
				os.Unsetenv(key)
			}

			result := getStringSliceEnv(key, tt.defaultValue)
			utils.AssertEqual(t, len(tt.expected), len(result), "Length should match")

			for i, expected := range tt.expected {
				utils.AssertEqual(t, expected, result[i], "Value at index %d should match", i)
			}
		})
	}
}

// clearEnv clears all environment variables that might affect the configuration
func clearEnv(t *testing.T) {
	envVars := []string{
		"PORT", "ENVIRONMENT", "SERVER_READ_TIMEOUT_SECONDS", "SERVER_WRITE_TIMEOUT_SECONDS",
		"SERVER_IDLE_TIMEOUT_SECONDS", "SERVER_MAX_HEADER_BYTES", "DISABLE_AUTH",
		"CORS_ALLOWED_ORIGINS", "DATABASE_URL", "DATABASE_MAX_OPEN_CONNS",
		"DATABASE_MAX_IDLE_CONNS", "DATABASE_CONN_MAX_LIFETIME",
		"DATABASE_CONN_MAX_IDLE_TIME", "GOOGLE_CLOUD_PROJECT",
		"PUBSUB_CLUSTER_EVENTS_TOPIC", "PUBSUB_EMULATOR_HOST",
		"GOOGLE_APPLICATION_CREDENTIALS", "PUBSUB_MAX_CONCURRENT_HANDLERS",
		"PUBSUB_MAX_OUTSTANDING_MESSAGES", "LOG_LEVEL", "LOG_FORMAT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
