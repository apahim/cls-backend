package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeForbidden    ErrorType = "forbidden"
	ErrorTypeInternal     ErrorType = "internal"
	ErrorTypeExternal     ErrorType = "external"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeUnavailable  ErrorType = "unavailable"
)

// Error codes
const (
	ErrCodeValidation   = "VALIDATION_FAILED"
	ErrCodeNotFound     = "RESOURCE_NOT_FOUND"
	ErrCodeConflict     = "RESOURCE_CONFLICT"
	ErrCodeInternal     = "INTERNAL_ERROR"
	ErrCodeExternal     = "EXTERNAL_ERROR"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden    = "FORBIDDEN"
)

// APIError represents a structured API error
type APIError struct {
	Type    ErrorType `json:"type"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
	TraceID string    `json:"trace_id,omitempty"`
}

// Error implements the error interface
func (e APIError) Error() string {
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// HTTPStatus returns the appropriate HTTP status code for the error type
func (e APIError) HTTPStatus() int {
	switch e.Type {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeUnauthorized:
		return http.StatusUnauthorized
	case ErrorTypeForbidden:
		return http.StatusForbidden
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeUnavailable:
		return http.StatusServiceUnavailable
	case ErrorTypeExternal:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// NewAPIError creates a new API error
func NewAPIError(errType ErrorType, code, message string) APIError {
	return APIError{
		Type:    errType,
		Code:    code,
		Message: message,
	}
}

// NewValidationError creates a validation error
func NewValidationError(code, message string, details any) APIError {
	return APIError{
		Type:    ErrorTypeValidation,
		Code:    code,
		Message: message,
		Details: details,
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, id string) APIError {
	return APIError{
		Type:    ErrorTypeNotFound,
		Code:    "RESOURCE_NOT_FOUND",
		Message: fmt.Sprintf("%s with id '%s' not found", resource, id),
	}
}

// NewConflictError creates a conflict error
func NewConflictError(code, message string) APIError {
	return APIError{
		Type:    ErrorTypeConflict,
		Code:    code,
		Message: message,
	}
}

// NewInternalError creates an internal error
func NewInternalError(code, message string) APIError {
	return APIError{
		Type:    ErrorTypeInternal,
		Code:    code,
		Message: message,
	}
}

// WrapError wraps a generic error as an APIError
func WrapError(err error, errType ErrorType, code string) APIError {
	return APIError{
		Type:    errType,
		Code:    code,
		Message: err.Error(),
	}
}

// ErrorHandler is a middleware for handling errors
func ErrorHandler() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		logger := NewLogger("error_handler")

		var apiError APIError

		switch e := err.(type) {
		case APIError:
			apiError = e
		case error:
			logger.Error("Unhandled error", zap.Error(e))
			apiError = NewInternalError("INTERNAL_ERROR", "An internal error occurred")
		default:
			logger.Error("Unknown error type", zap.Any("error", err))
			apiError = NewInternalError("UNKNOWN_ERROR", "An unknown error occurred")
		}

		// Add trace ID if available
		if traceID := c.GetHeader("X-Trace-Id"); traceID != "" {
			apiError.TraceID = traceID
		}

		logger.Error("API Error",
			zap.String("type", string(apiError.Type)),
			zap.String("code", apiError.Code),
			zap.String("message", apiError.Message),
			zap.String("trace_id", apiError.TraceID),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
		)

		c.JSON(apiError.HTTPStatus(), gin.H{"error": apiError})
		c.Abort()
	})
}

// HandleError is a helper function to handle errors in handlers
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var apiError APIError

	switch e := err.(type) {
	case APIError:
		apiError = e
	default:
		apiError = WrapError(err, ErrorTypeInternal, "INTERNAL_ERROR")
	}

	// Add trace ID if available
	if traceID := c.GetHeader("X-Trace-Id"); traceID != "" {
		apiError.TraceID = traceID
	}

	c.JSON(apiError.HTTPStatus(), gin.H{"error": apiError})
	c.Abort()
}

// ValidationDetails represents validation error details
type ValidationDetails struct {
	Field   string `json:"field"`
	Value   any    `json:"value,omitempty"`
	Message string `json:"message"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationDetails

// Error implements the error interface
func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return "validation failed"
	}

	data, _ := json.Marshal(v)
	return fmt.Sprintf("validation failed: %s", string(data))
}

// NewValidationErrors creates a new validation errors collection
func NewValidationErrors(errors ...ValidationDetails) ValidationErrors {
	return ValidationErrors(errors)
}

// Add adds a validation error
func (v *ValidationErrors) Add(field, message string, value any) {
	*v = append(*v, ValidationDetails{
		Field:   field,
		Value:   value,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

// ToAPIError converts validation errors to an API error
func (v ValidationErrors) ToAPIError() APIError {
	return NewValidationError("VALIDATION_FAILED", "Request validation failed", v)
}

// IsPostgreSQLUniqueConstraintViolation checks if an error is a PostgreSQL unique constraint violation
func IsPostgreSQLUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key value violates unique constraint") ||
		strings.Contains(errStr, "pq: duplicate key value violates unique constraint")
}

// IsClusterNameConflict checks if an error is specifically a cluster name conflict
func IsClusterNameConflict(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return IsPostgreSQLUniqueConstraintViolation(err) &&
		(strings.Contains(errStr, "clusters_name_created_by_key") ||
			strings.Contains(errStr, "clusters_name_key"))
}

// ConvertDBError converts database errors to appropriate API errors
func ConvertDBError(err error) APIError {
	if err == nil {
		return APIError{}
	}

	// Check for cluster name conflicts
	if IsClusterNameConflict(err) {
		return NewConflictError("CLUSTER_NAME_EXISTS", "A cluster with this name already exists")
	}

	// Check for other unique constraint violations
	if IsPostgreSQLUniqueConstraintViolation(err) {
		return NewConflictError("RESOURCE_CONFLICT", "Resource already exists")
	}

	// Default to internal error
	return NewInternalError("DATABASE_ERROR", "Database operation failed")
}
