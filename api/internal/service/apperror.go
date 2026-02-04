package core

import (
	"errors"
	"fmt"
)

// 错误代码常量
const (
	AppErrValidation     = "VALIDATION_ERROR"
	AppErrNotFound       = "NOT_FOUND"
	AppErrDatabase       = "DATABASE_ERROR"
	AppErrCachePoolEmpty = "CACHE_POOL_EMPTY"
	AppErrUnauthorized   = "UNAUTHORIZED"
	AppErrForbidden      = "FORBIDDEN"
	AppErrInternal       = "INTERNAL_ERROR"
)

// ServiceError 服务层应用错误类型
type ServiceError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Err     error                  `json:"-"`
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a validation error
func NewValidationError(field string, reason string) *ServiceError {
	return &ServiceError{
		Code:    AppErrValidation,
		Message: "Validation failed",
		Details: map[string]interface{}{field: reason},
	}
}

// NewDatabaseError creates a database error
func NewDatabaseError(err error) *ServiceError {
	return &ServiceError{
		Code:    AppErrDatabase,
		Message: "Database operation failed",
		Err:     err,
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string, id interface{}) *ServiceError {
	return &ServiceError{
		Code:    AppErrNotFound,
		Message: fmt.Sprintf("%s not found: %v", resource, id),
	}
}

// NewCachePoolEmptyError creates a cache pool empty error
func NewCachePoolEmptyError(poolType string, groupID int) *ServiceError {
	return &ServiceError{
		Code:    AppErrCachePoolEmpty,
		Message: fmt.Sprintf("Cache pool empty: %s (group %d)", poolType, groupID),
		Details: map[string]interface{}{
			"pool_type": poolType,
			"group_id":  groupID,
		},
	}
}

// NewInternalError creates an internal server error
func NewInternalError(err error) *ServiceError {
	return &ServiceError{
		Code:    AppErrInternal,
		Message: "Internal server error",
		Err:     err,
	}
}

// IsServiceError checks if an error is a ServiceError
func IsServiceError(err error) bool {
	var svcErr *ServiceError
	return errors.As(err, &svcErr)
}

// GetServiceError extracts ServiceError from error
func GetServiceError(err error) *ServiceError {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		return svcErr
	}
	return nil
}
