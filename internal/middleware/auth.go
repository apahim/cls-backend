package middleware

import (
	"net/http"
	"strings"

	"github.com/apahim/cls-backend/internal/auth"
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

		// Create user context with access control information
		userCtx := auth.NewUserContext(userEmail)

		// Set user context in Gin context for handlers to use
		c.Set("user_context", userCtx)
		// Keep user_email for backward compatibility (if needed)
		c.Set("user_email", userEmail)

		// Log the authenticated request with access level
		accessLevel := "user"
		if userCtx.IsController {
			accessLevel = "controller"
		}

		zap.L().Debug("Authenticated request",
			zap.String("user_email", userEmail),
			zap.String("access_level", accessLevel),
			zap.Bool("is_controller", userCtx.IsController),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
		)

		c.Next()
	}
}

// MockUserContext provides a mock user context for development
func MockUserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set a mock user email for development - use controller email for status updates
		userEmail := "controller@system.local"
		userCtx := auth.NewUserContext(userEmail)

		c.Set("user_context", userCtx)
		c.Set("user_email", userEmail)
		c.Next()
	}
}

// GetUserContext extracts the user context from Gin context
func GetUserContext(c *gin.Context) (*auth.UserContext, bool) {
	userCtx, exists := c.Get("user_context")
	if !exists {
		return nil, false
	}

	authUserCtx, ok := userCtx.(*auth.UserContext)
	return authUserCtx, ok
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