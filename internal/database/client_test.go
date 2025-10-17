package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/utils"
)

func TestNewClient(t *testing.T) {
	utils.SkipIfNoTestDB(t)

	cfg := config.DatabaseConfig{
		URL:             utils.GetTestDBURL(),
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, false, "Should create client without error")
	utils.AssertNotNil(t, client, "Client should not be nil")

	defer client.Close()

	// Test that client is properly configured
	stats := client.Stats()
	utils.AssertEqual(t, 10, stats.MaxOpenConnections, "Max open connections should match config")

	// Test ping
	ctx := context.Background()
	err = client.Ping(ctx)
	utils.AssertError(t, err, false, "Should ping successfully")
}

func TestNewClient_InvalidURL(t *testing.T) {
	cfg := config.DatabaseConfig{
		URL:             "invalid-url",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, true, "Should return error for invalid URL")
	utils.AssertNil(t, client, "Client should be nil on error")
}

func TestClient_Transaction(t *testing.T) {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, false, "Should create client")
	defer client.Close()

	// Create a test table
	ctx := context.Background()
	_, err = client.ExecContext(ctx, "CREATE TABLE test_table (id SERIAL PRIMARY KEY, name TEXT)")
	utils.AssertError(t, err, false, "Should create test table")

	// Test successful transaction
	err = client.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test_table (name) VALUES ($1)", "test1")
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO test_table (name) VALUES ($1)", "test2")
		return err
	})
	utils.AssertError(t, err, false, "Transaction should succeed")

	// Verify data was inserted
	var count int
	err = client.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_table").Scan(&count)
	utils.AssertError(t, err, false, "Should count rows")
	utils.AssertEqual(t, 2, count, "Should have 2 rows after successful transaction")

	// Test failed transaction (should rollback)
	err = client.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test_table (name) VALUES ($1)", "test3")
		if err != nil {
			return err
		}
		// Force an error
		_, err = tx.Exec("INSERT INTO invalid_table (name) VALUES ($1)", "test4")
		return err
	})
	utils.AssertError(t, err, true, "Transaction should fail")

	// Verify data was rolled back
	err = client.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_table").Scan(&count)
	utils.AssertError(t, err, false, "Should count rows")
	utils.AssertEqual(t, 2, count, "Should still have 2 rows after failed transaction")
}

func TestClient_QueryOperations(t *testing.T) {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, false, "Should create client")
	defer client.Close()

	ctx := context.Background()

	// Test ExecContext
	result, err := client.ExecContext(ctx, "CREATE TABLE test_users (id SERIAL PRIMARY KEY, name TEXT, age INT)")
	utils.AssertError(t, err, false, "Should create table")
	utils.AssertNotNil(t, result, "Result should not be nil")

	// Test insert
	result, err = client.ExecContext(ctx, "INSERT INTO test_users (name, age) VALUES ($1, $2)", "Alice", 30)
	utils.AssertError(t, err, false, "Should insert row")
	rowsAffected, err := result.RowsAffected()
	utils.AssertError(t, err, false, "Should get rows affected")
	utils.AssertEqual(t, int64(1), rowsAffected, "Should affect 1 row")

	// Test QueryRowContext
	var name string
	var age int
	err = client.QueryRowContext(ctx, "SELECT name, age FROM test_users WHERE name = $1", "Alice").Scan(&name, &age)
	utils.AssertError(t, err, false, "Should query single row")
	utils.AssertEqual(t, "Alice", name, "Name should match")
	utils.AssertEqual(t, 30, age, "Age should match")

	// Test QueryContext
	_, err = client.ExecContext(ctx, "INSERT INTO test_users (name, age) VALUES ($1, $2)", "Bob", 25)
	utils.AssertError(t, err, false, "Should insert second row")

	rows, err := client.QueryContext(ctx, "SELECT name, age FROM test_users ORDER BY name")
	utils.AssertError(t, err, false, "Should query multiple rows")
	defer rows.Close()

	var users []struct {
		Name string
		Age  int
	}

	for rows.Next() {
		var user struct {
			Name string
			Age  int
		}
		err := rows.Scan(&user.Name, &user.Age)
		utils.AssertError(t, err, false, "Should scan row")
		users = append(users, user)
	}

	utils.AssertError(t, rows.Err(), false, "Should not have iteration error")
	utils.AssertEqual(t, 2, len(users), "Should have 2 users")
	utils.AssertEqual(t, "Alice", users[0].Name, "First user should be Alice")
	utils.AssertEqual(t, "Bob", users[1].Name, "Second user should be Bob")
}

func TestClient_Health(t *testing.T) {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, false, "Should create client")
	defer client.Close()

	ctx := context.Background()
	health, err := client.Health(ctx)
	utils.AssertError(t, err, false, "Should get health status")
	utils.AssertNotNil(t, health, "Health status should not be nil")
	utils.AssertEqual(t, "healthy", health.Status, "Should be healthy")
	utils.AssertEqual(t, 10, health.MaxOpenConns, "Max open connections should match")
	utils.AssertNil(t, health.PingError, "Ping error should be nil")
}

func TestClient_HealthWithHighLoad(t *testing.T) {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    2, // Low limit to trigger degraded status
		MaxIdleConns:    1,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, false, "Should create client")
	defer client.Close()

	// Create multiple connections to potentially exhaust the pool
	ctx := context.Background()

	// Start a long-running transaction to hold a connection
	tx, err := client.db.BeginTx(ctx, nil)
	utils.AssertError(t, err, false, "Should begin transaction")
	defer tx.Rollback()

	// Get health status
	health, err := client.Health(ctx)
	utils.AssertError(t, err, false, "Should get health status")
	utils.AssertNotNil(t, health, "Health status should not be nil")

	// The status might be healthy or degraded depending on timing
	utils.AssertTrue(t, health.Status == "healthy" || health.Status == "degraded",
		"Status should be healthy or degraded, got: %s", health.Status)
}

func TestClient_Stats(t *testing.T) {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, false, "Should create client")
	defer client.Close()

	stats := client.Stats()
	utils.AssertEqual(t, 10, stats.MaxOpenConnections, "Max open connections should match")
	utils.AssertTrue(t, stats.OpenConnections >= 0, "Open connections should be non-negative")
	utils.AssertTrue(t, stats.InUse >= 0, "In use connections should be non-negative")
	utils.AssertTrue(t, stats.Idle >= 0, "Idle connections should be non-negative")
}

func TestClient_Close(t *testing.T) {
	utils.SkipIfNoTestDB(t)

	testDBURL := utils.SetupTestDB(t)
	cfg := config.DatabaseConfig{
		URL:             testDBURL,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	client, err := NewClient(cfg)
	utils.AssertError(t, err, false, "Should create client")

	// Use the client
	ctx := context.Background()
	err = client.Ping(ctx)
	utils.AssertError(t, err, false, "Should ping successfully")

	// Close the client
	err = client.Close()
	utils.AssertError(t, err, false, "Should close without error")

	// Verify client is closed by attempting to ping
	err = client.Ping(ctx)
	utils.AssertError(t, err, true, "Should fail to ping after close")
}
