package utils

import (
	"context"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	Jitter          bool
	RetryableErrors []string
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context, attempt int) error

// IsRetryableFunc determines if an error should trigger a retry
type IsRetryableFunc func(err error) bool

// RetryResult contains the result of a retry operation
type RetryResult struct {
	Attempts int
	Duration time.Duration
	LastErr  error
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config RetryConfig, fn RetryableFunc, isRetryable IsRetryableFunc) RetryResult {
	logger := NewLogger("retry")
	start := time.Now()

	var lastErr error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := fn(ctx, attempt)
		if err == nil {
			return RetryResult{
				Attempts: attempt,
				Duration: time.Since(start),
			}
		}

		lastErr = err

		// Check if we should retry
		if !isRetryable(err) {
			logger.Debug("Error is not retryable",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			break
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		delay := calculateDelay(config, attempt)

		logger.Debug("Retrying after error",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay),
		)

		select {
		case <-ctx.Done():
			return RetryResult{
				Attempts: attempt,
				Duration: time.Since(start),
				LastErr:  ctx.Err(),
			}
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return RetryResult{
		Attempts: config.MaxAttempts,
		Duration: time.Since(start),
		LastErr:  lastErr,
	}
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(config RetryConfig, attempt int) time.Duration {
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))

	// Apply jitter if enabled
	if config.Jitter {
		jitter := rand.Float64() * 0.1 // 10% jitter
		delay = delay * (1 + jitter)
	}

	// Ensure delay doesn't exceed max delay
	if time.Duration(delay) > config.MaxDelay {
		delay = float64(config.MaxDelay)
	}

	return time.Duration(delay)
}

// RetryWithBackoff is a simple retry function with exponential backoff
func RetryWithBackoff(ctx context.Context, maxRetries int, initialDelay time.Duration, fn func() error) error {
	config := RetryConfig{
		MaxAttempts:   maxRetries,
		InitialDelay:  initialDelay,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}

	retryableFunc := func(ctx context.Context, attempt int) error {
		return fn()
	}

	isRetryableFunc := func(err error) bool {
		return true // Retry all errors
	}

	result := Retry(ctx, config, retryableFunc, isRetryableFunc)
	return result.LastErr
}

// IsTemporaryError checks if an error is temporary and should be retried
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	// Check for temporary interface
	if temp, ok := err.(interface{ Temporary() bool }); ok {
		return temp.Temporary()
	}

	// Check for timeout interface
	if timeout, ok := err.(interface{ Timeout() bool }); ok {
		return timeout.Timeout()
	}

	// Check for context errors
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false // Don't retry context errors
	}

	// Add more specific error checks as needed
	errorMsg := err.Error()
	temporaryErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no such host",
		"timeout",
		"deadline exceeded",
		"rate limit",
		"service unavailable",
		"internal server error",
	}

	for _, tempErr := range temporaryErrors {
		if contains(errorMsg, tempErr) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	return len(str) >= len(substr) &&
		   (str == substr ||
		    (len(str) > len(substr) &&
		     findSubstring(str, substr)))
}

func findSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RetryPolicy defines a retry policy for specific operations
type RetryPolicy struct {
	Name            string
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	Jitter          bool
	RetryableErrors []string
}

// NewRetryPolicy creates a new retry policy
func NewRetryPolicy(name string) *RetryPolicy {
	return &RetryPolicy{
		Name:          name,
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// WithMaxAttempts sets the maximum number of attempts
func (p *RetryPolicy) WithMaxAttempts(attempts int) *RetryPolicy {
	p.MaxAttempts = attempts
	return p
}

// WithInitialDelay sets the initial delay
func (p *RetryPolicy) WithInitialDelay(delay time.Duration) *RetryPolicy {
	p.InitialDelay = delay
	return p
}

// WithMaxDelay sets the maximum delay
func (p *RetryPolicy) WithMaxDelay(delay time.Duration) *RetryPolicy {
	p.MaxDelay = delay
	return p
}

// WithBackoffFactor sets the backoff factor
func (p *RetryPolicy) WithBackoffFactor(factor float64) *RetryPolicy {
	p.BackoffFactor = factor
	return p
}

// WithJitter enables or disables jitter
func (p *RetryPolicy) WithJitter(enabled bool) *RetryPolicy {
	p.Jitter = enabled
	return p
}

// Execute executes a function with this retry policy
func (p *RetryPolicy) Execute(ctx context.Context, fn RetryableFunc, isRetryable IsRetryableFunc) RetryResult {
	config := RetryConfig{
		MaxAttempts:     p.MaxAttempts,
		InitialDelay:    p.InitialDelay,
		MaxDelay:        p.MaxDelay,
		BackoffFactor:   p.BackoffFactor,
		Jitter:          p.Jitter,
		RetryableErrors: p.RetryableErrors,
	}

	return Retry(ctx, config, fn, isRetryable)
}