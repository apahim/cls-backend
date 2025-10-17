package middleware

import (
	"net/http"
	"strings"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthRequired middleware enforces authentication
func AuthRequired(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if authentication is disabled (for development)
		if !cfg.Auth.Enabled {
			c.Next()
			return
		}

		// Extract user email from header (set by API Gateway)
		userEmail := c.GetHeader("X-User-Email")
		if userEmail == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "AUTH_REQUIRED",
			})
			c.Abort()
			return
		}

		// Validate email format (basic validation)
		if !isValidEmail(userEmail) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid user email format",
				"code":  "INVALID_EMAIL",
			})
			c.Abort()
			return
		}

		// Set user email in context for handlers to use
		c.Set("user_email", userEmail)

		// Log the authenticated request
		zap.L().Debug("Authenticated request",
			zap.String("user_email", userEmail),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
		)

		c.Next()
	}
}

// MockUserContext provides a mock user context for development
func MockUserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set a mock user email for development
		c.Set("user_email", "dev-user@example.com")
		c.Next()
	}
}

// isValidEmail performs basic email validation
func isValidEmail(email string) bool {
	// Basic email validation - contains @ and has parts before and after
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	// Must contain a dot in the domain part
	return strings.Contains(parts[1], ".")
}