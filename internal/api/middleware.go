package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)

		c.Next()
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(logger *utils.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		requestID, _ := param.Keys["request_id"].(string)

		logger.Info("HTTP Request",
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.Int("status", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.String("client_ip", param.ClientIP),
			zap.String("user_agent", param.Request.UserAgent()),
			zap.String("request_id", requestID),
			zap.Int("body_size", param.BodySize),
		)

		return ""
	})
}

// CORSMiddleware handles CORS headers
func CORSMiddleware(cfg config.ServerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Set CORS headers
		if len(cfg.CorsAllowedOrigins) > 0 {
			for _, allowedOrigin := range cfg.CorsAllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// AuthMiddleware handles authentication and authorization
func AuthMiddleware(authCfg config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for health checks and public endpoints
		if isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// For development, we might want to skip auth
		if !authCfg.Enabled {
			c.Set("user_id", "dev-user")
			c.Set("user_email", "dev@example.com")
			c.Next()
			return
		}

		// Get authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, utils.NewAPIError(
				utils.ErrCodeUnauthorized,
				"Missing authorization header",
				"",
			))
			c.Abort()
			return
		}

		// Extract bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, utils.NewAPIError(
				utils.ErrCodeUnauthorized,
				"Invalid authorization header format",
				"",
			))
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token (this would integrate with OAuth 2.0/OIDC provider)
		userInfo, err := validateToken(token, authCfg)
		if err != nil {
			c.JSON(http.StatusUnauthorized, utils.NewAPIError(
				utils.ErrCodeUnauthorized,
				"Invalid or expired token",
				err.Error(),
			))
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", userInfo.ID)
		c.Set("user_email", userInfo.Email)
		c.Set("user_roles", userInfo.Roles)

		c.Next()
	}
}

// RateLimitMiddleware implements basic rate limiting
func RateLimitMiddleware(cfg config.ServerConfig) gin.HandlerFunc {
	// This is a simple in-memory rate limiter
	// In production, you'd want to use Redis or similar
	return func(c *gin.Context) {
		// For now, just pass through
		// In a real implementation, you'd track requests per client
		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}

// ErrorHandlingMiddleware handles panics and errors
func ErrorHandlingMiddleware(logger *utils.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		requestID, _ := c.Get("request_id")

		logger.Error("Panic recovered",
			zap.Any("panic", recovered),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Any("request_id", requestID),
		)

		c.JSON(http.StatusInternalServerError, utils.NewAPIError(
			utils.ErrCodeInternal,
			"Internal server error",
			"An unexpected error occurred",
		))
	})
}

// TimeoutMiddleware adds request timeout
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set a timeout on the request context
		ctx := c.Request.Context()
		if timeout > 0 {
			var cancel func()
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}

// UserInfo represents authenticated user information
type UserInfo struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// validateToken validates an OAuth 2.0/OIDC token
func validateToken(token string, authCfg config.AuthConfig) (*UserInfo, error) {
	// In a real implementation, this would:
	// 1. Validate the JWT signature
	// 2. Check token expiration
	// 3. Verify the issuer
	// 4. Extract user claims

	// For development/testing purposes, we'll do basic validation
	if !authCfg.Enabled {
		return &UserInfo{
			ID:    "dev-user",
			Email: "dev@example.com",
			Roles: []string{"admin"},
		}, nil
	}

	// TODO: Implement proper JWT/OIDC validation
	// This would typically use a library like go-oidc or go-jwt
	if token == "" {
		return nil, errors.New("empty token")
	}

	// Mock validation for now
	return &UserInfo{
		ID:    "user-123",
		Email: "user@example.com",
		Roles: []string{"user"},
	}, nil
}

// isPublicEndpoint checks if an endpoint should be publicly accessible
func isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/health",
		"/status/health",
		"/metrics",
		"/docs",
		"/swagger",
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}
