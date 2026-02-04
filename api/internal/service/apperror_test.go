package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError(t *testing.T) {
	t.Run("database error", func(t *testing.T) {
		dbErr := errors.New("connection failed")
		appErr := NewDatabaseError(dbErr)

		assert.Equal(t, AppErrDatabase, appErr.Code)
		assert.Contains(t, appErr.Error(), "Database operation failed")
		assert.NotNil(t, appErr.Err)
	})

	t.Run("validation error", func(t *testing.T) {
		appErr := NewValidationError("email", "invalid format")

		assert.Equal(t, AppErrValidation, appErr.Code)
		assert.Contains(t, appErr.Error(), "Validation failed")
		assert.NotNil(t, appErr.Details)
	})

	t.Run("not found error", func(t *testing.T) {
		appErr := NewNotFoundError("Site", 123)

		assert.Equal(t, AppErrNotFound, appErr.Code)
		assert.Contains(t, appErr.Error(), "Site")
		assert.Contains(t, appErr.Error(), "123")
	})
}

func TestErrorChaining(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := NewDatabaseError(originalErr)

	assert.True(t, errors.Is(appErr, originalErr))
}

func TestErrorConstructors(t *testing.T) {
	t.Run("NewValidationError", func(t *testing.T) {
		appErr := NewValidationError("username", "too short")

		assert.Equal(t, AppErrValidation, appErr.Code)
		assert.Equal(t, "Validation failed", appErr.Message)
		assert.NotNil(t, appErr.Details)
		assert.Equal(t, "too short", appErr.Details["username"])
	})

	t.Run("NewDatabaseError", func(t *testing.T) {
		originalErr := errors.New("query timeout")
		appErr := NewDatabaseError(originalErr)

		assert.Equal(t, AppErrDatabase, appErr.Code)
		assert.Equal(t, "Database operation failed", appErr.Message)
		assert.Equal(t, originalErr, appErr.Err)
	})

	t.Run("NewNotFoundError", func(t *testing.T) {
		appErr := NewNotFoundError("User", 456)

		assert.Equal(t, AppErrNotFound, appErr.Code)
		assert.Contains(t, appErr.Message, "User not found")
		assert.Contains(t, appErr.Message, "456")
	})

	t.Run("NewCachePoolEmptyError", func(t *testing.T) {
		appErr := NewCachePoolEmptyError("keyword", 10)

		assert.Equal(t, AppErrCachePoolEmpty, appErr.Code)
		assert.Contains(t, appErr.Message, "Cache pool empty")
		assert.Contains(t, appErr.Message, "keyword")
		assert.Contains(t, appErr.Message, "10")
		assert.NotNil(t, appErr.Details)
		assert.Equal(t, "keyword", appErr.Details["pool_type"])
		assert.Equal(t, 10, appErr.Details["group_id"])
	})

	t.Run("NewInternalError", func(t *testing.T) {
		originalErr := errors.New("panic recovered")
		appErr := NewInternalError(originalErr)

		assert.Equal(t, AppErrInternal, appErr.Code)
		assert.Equal(t, "Internal server error", appErr.Message)
		assert.Equal(t, originalErr, appErr.Err)
	})
}

func TestIsServiceError(t *testing.T) {
	t.Run("returns true for AppError", func(t *testing.T) {
		appErr := NewValidationError("field", "reason")
		assert.True(t, IsServiceError(appErr))
	})

	t.Run("returns false for standard error", func(t *testing.T) {
		err := errors.New("standard error")
		assert.False(t, IsServiceError(err))
	})

	t.Run("returns false for nil", func(t *testing.T) {
		assert.False(t, IsServiceError(nil))
	})
}

func TestGetServiceError(t *testing.T) {
	t.Run("extracts AppError", func(t *testing.T) {
		appErr := NewDatabaseError(errors.New("db error"))
		extracted := GetServiceError(appErr)

		assert.NotNil(t, extracted)
		assert.Equal(t, appErr.Code, extracted.Code)
		assert.Equal(t, appErr.Message, extracted.Message)
	})

	t.Run("returns nil for standard error", func(t *testing.T) {
		err := errors.New("standard error")
		extracted := GetServiceError(err)

		assert.Nil(t, extracted)
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		extracted := GetServiceError(nil)
		assert.Nil(t, extracted)
	})
}

func TestAppErrorUnwrap(t *testing.T) {
	t.Run("unwraps to original error", func(t *testing.T) {
		originalErr := errors.New("original error")
		appErr := NewDatabaseError(originalErr)

		unwrapped := errors.Unwrap(appErr)
		assert.Equal(t, originalErr, unwrapped)
	})

	t.Run("returns nil when no wrapped error", func(t *testing.T) {
		appErr := NewValidationError("field", "reason")

		unwrapped := errors.Unwrap(appErr)
		assert.Nil(t, unwrapped)
	})
}
