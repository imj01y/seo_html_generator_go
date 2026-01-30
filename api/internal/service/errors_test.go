// Package core provides error handling tests
package core

import (
	"errors"
	"net/http"
	"testing"
)

// TestAppError_Error tests the error message format
func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name: "error without detail",
			err: &AppError{
				Code:    ErrNotFound,
				Message: "资源不存在",
			},
			expected: "[1004] 资源不存在",
		},
		{
			name: "error with detail",
			err: &AppError{
				Code:    ErrNotFound,
				Message: "资源不存在",
				Detail:  "用户ID: 123",
			},
			expected: "[1004] 资源不存在: 用户ID: 123",
		},
		{
			name: "error with empty detail",
			err: &AppError{
				Code:    ErrInvalidParam,
				Message: "参数无效",
				Detail:  "",
			},
			expected: "[1001] 参数无效",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestAppError_HTTPStatus tests HTTP status code mapping
func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected int
	}{
		// Common errors
		{"success", ErrSuccess, http.StatusOK},
		{"unknown", ErrUnknown, http.StatusInternalServerError},
		{"invalid param", ErrInvalidParam, http.StatusBadRequest},
		{"unauthorized", ErrUnauthorized, http.StatusUnauthorized},
		{"forbidden", ErrForbidden, http.StatusForbidden},
		{"not found", ErrNotFound, http.StatusNotFound},
		{"method not allowed", ErrMethodNotAllow, http.StatusMethodNotAllowed},
		{"too many requests", ErrTooManyRequests, http.StatusTooManyRequests},
		{"internal server", ErrInternalServer, http.StatusInternalServerError},
		{"timeout", ErrTimeout, http.StatusGatewayTimeout},
		{"validation", ErrValidation, http.StatusUnprocessableEntity},

		// Database errors
		{"db connection", ErrDBConnection, http.StatusServiceUnavailable},
		{"db query", ErrDBQuery, http.StatusInternalServerError},
		{"db duplicate", ErrDBDuplicate, http.StatusConflict},
		{"db not found", ErrDBNotFound, http.StatusNotFound},

		// Cache errors
		{"cache connection", ErrCacheConnection, http.StatusServiceUnavailable},
		{"cache expired", ErrCacheExpired, http.StatusGone},
		{"cache full", ErrCacheFull, http.StatusInsufficientStorage},
		{"cache miss", ErrCacheMiss, http.StatusNotFound},

		// Template errors
		{"template not found", ErrTemplateNotFound, http.StatusNotFound},
		{"template invalid", ErrTemplateInvalid, http.StatusBadRequest},
		{"template syntax", ErrTemplateSyntax, http.StatusBadRequest},

		// Pool errors
		{"pool exhausted", ErrPoolExhausted, http.StatusServiceUnavailable},
		{"pool timeout", ErrPoolTimeout, http.StatusServiceUnavailable},

		// Site errors
		{"site not found", ErrSiteNotFound, http.StatusNotFound},
		{"site disabled", ErrSiteDisabled, http.StatusForbidden},

		// Scheduler errors
		{"scheduler not running", ErrSchedulerNotRunning, http.StatusServiceUnavailable},
		{"scheduler task exist", ErrSchedulerTaskExist, http.StatusConflict},
		{"scheduler timeout", ErrSchedulerTimeout, http.StatusGatewayTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.code)
			status := err.HTTPStatus()
			if status != tt.expected {
				t.Errorf("HTTPStatus() for code %d = %d, want %d", tt.code, status, tt.expected)
			}
		})
	}
}

// TestAppError_HTTPStatus_UnknownCode tests that unknown codes return 500
func TestAppError_HTTPStatus_UnknownCode(t *testing.T) {
	err := &AppError{
		Code:    ErrorCode(99999), // Unknown code
		Message: "unknown error",
	}
	status := err.HTTPStatus()
	if status != http.StatusInternalServerError {
		t.Errorf("HTTPStatus() for unknown code = %d, want %d", status, http.StatusInternalServerError)
	}
}

// TestNewErrorWithDetail tests creating error with detail message
func TestNewErrorWithDetail(t *testing.T) {
	tests := []struct {
		name           string
		code           ErrorCode
		detail         string
		expectedCode   ErrorCode
		expectedDetail string
	}{
		{
			name:           "with detail",
			code:           ErrInvalidParam,
			detail:         "字段 'name' 不能为空",
			expectedCode:   ErrInvalidParam,
			expectedDetail: "字段 'name' 不能为空",
		},
		{
			name:           "empty detail",
			code:           ErrNotFound,
			detail:         "",
			expectedCode:   ErrNotFound,
			expectedDetail: "",
		},
		{
			name:           "long detail",
			code:           ErrDBQuery,
			detail:         "SQL error: duplicate key value violates unique constraint",
			expectedCode:   ErrDBQuery,
			expectedDetail: "SQL error: duplicate key value violates unique constraint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrorWithDetail(tt.code, tt.detail)
			if err.Code != tt.expectedCode {
				t.Errorf("Code = %d, want %d", err.Code, tt.expectedCode)
			}
			if err.Detail != tt.expectedDetail {
				t.Errorf("Detail = %q, want %q", err.Detail, tt.expectedDetail)
			}
			// Message should be set from errorMessages
			if err.Message == "" {
				t.Error("Message should not be empty")
			}
		})
	}
}

// TestNewErrorWithErr tests creating error wrapping an existing error
func TestNewErrorWithErr(t *testing.T) {
	tests := []struct {
		name         string
		code         ErrorCode
		originalErr  error
		wantDetail   bool
		wantOriginal bool
	}{
		{
			name:         "with original error",
			code:         ErrDBQuery,
			originalErr:  errors.New("connection refused"),
			wantDetail:   true,
			wantOriginal: true,
		},
		{
			name:         "with nil error",
			code:         ErrInternalServer,
			originalErr:  nil,
			wantDetail:   false,
			wantOriginal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrorWithErr(tt.code, tt.originalErr)
			if err.Code != tt.code {
				t.Errorf("Code = %d, want %d", err.Code, tt.code)
			}

			if tt.wantDetail {
				if err.Detail == "" {
					t.Error("Detail should be set from original error")
				}
				if err.Detail != tt.originalErr.Error() {
					t.Errorf("Detail = %q, want %q", err.Detail, tt.originalErr.Error())
				}
			} else {
				if err.Detail != "" {
					t.Errorf("Detail should be empty, got %q", err.Detail)
				}
			}

			if tt.wantOriginal {
				if err.Err == nil {
					t.Error("Err should be set")
				}
				// Test Unwrap
				if err.Unwrap() != tt.originalErr {
					t.Error("Unwrap() should return original error")
				}
			} else {
				if err.Err != nil {
					t.Error("Err should be nil")
				}
			}
		})
	}
}

// TestIsAppError tests error type checking
func TestIsAppError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "AppError pointer",
			err:      NewError(ErrNotFound),
			expected: true,
		},
		{
			name:     "AppError with detail",
			err:      NewErrorWithDetail(ErrInvalidParam, "test"),
			expected: true,
		},
		{
			name:     "AppError with wrapped error",
			err:      NewErrorWithErr(ErrDBQuery, errors.New("db error")),
			expected: true,
		},
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAppError(tt.err)
			if result != tt.expected {
				t.Errorf("IsAppError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetAppError tests extracting AppError from an error
func TestGetAppError(t *testing.T) {
	appErr := NewError(ErrNotFound)
	standardErr := errors.New("standard error")

	tests := []struct {
		name     string
		err      error
		wantNil  bool
		wantCode ErrorCode
	}{
		{
			name:     "AppError",
			err:      appErr,
			wantNil:  false,
			wantCode: ErrNotFound,
		},
		{
			name:    "standard error",
			err:     standardErr,
			wantNil: true,
		},
		{
			name:    "nil error",
			err:     nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAppError(tt.err)
			if tt.wantNil {
				if result != nil {
					t.Errorf("GetAppError() should return nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Error("GetAppError() should not return nil")
				} else if result.Code != tt.wantCode {
					t.Errorf("GetAppError().Code = %d, want %d", result.Code, tt.wantCode)
				}
			}
		})
	}
}

// TestErrorMessages tests that all error codes have messages
func TestErrorMessages(t *testing.T) {
	// All defined error codes
	allCodes := []ErrorCode{
		// Common errors
		ErrSuccess, ErrUnknown, ErrInvalidParam, ErrUnauthorized, ErrForbidden,
		ErrNotFound, ErrMethodNotAllow, ErrTooManyRequests, ErrInternalServer,
		ErrTimeout, ErrValidation,

		// Database errors
		ErrDBConnection, ErrDBQuery, ErrDBInsert, ErrDBUpdate, ErrDBDelete,
		ErrDBDuplicate, ErrDBNotFound, ErrDBTxBegin, ErrDBTxCommit, ErrDBTxRollback,

		// Cache errors
		ErrCacheConnection, ErrCacheGet, ErrCacheSet, ErrCacheDelete,
		ErrCacheExpired, ErrCacheFull, ErrCacheMiss, ErrCacheInvalid,

		// Template errors
		ErrTemplateNotFound, ErrTemplateParse, ErrTemplateRender, ErrTemplateInvalid,
		ErrTemplateCompile, ErrTemplateSyntax, ErrTemplateExecution, ErrTemplateDataType,

		// Pool errors
		ErrPoolExhausted, ErrPoolTimeout, ErrPoolClosed, ErrPoolInvalid,
		ErrPoolOverflow, ErrPoolGetFailed, ErrPoolPutFailed,

		// Site errors
		ErrSiteNotFound, ErrSiteDisabled, ErrSiteInvalid, ErrSiteDomain,
		ErrSiteConfig, ErrSiteTemplate, ErrSiteGroup, ErrSitePermission,

		// Scheduler errors
		ErrSchedulerNotRunning, ErrSchedulerTaskExist, ErrSchedulerTaskNotFound,
		ErrSchedulerInvalidCron, ErrSchedulerExecFailed, ErrSchedulerQueueFull,
		ErrSchedulerTimeout, ErrSchedulerCancelled,
	}

	for _, code := range allCodes {
		t.Run(GetErrorMessage(code), func(t *testing.T) {
			msg := GetErrorMessage(code)
			if msg == "" {
				t.Errorf("Error code %d has no message", code)
			}
			// Verify the message is not just the unknown error message (unless it's ErrUnknown)
			if code != ErrUnknown && msg == GetErrorMessage(ErrUnknown) {
				t.Errorf("Error code %d has fallback unknown message, should have specific message", code)
			}
		})
	}
}

// TestGetHTTPStatus tests the GetHTTPStatus helper function
func TestGetHTTPStatus(t *testing.T) {
	// Test known codes
	if status := GetHTTPStatus(ErrNotFound); status != http.StatusNotFound {
		t.Errorf("GetHTTPStatus(ErrNotFound) = %d, want %d", status, http.StatusNotFound)
	}

	// Test unknown code returns 500
	if status := GetHTTPStatus(ErrorCode(99999)); status != http.StatusInternalServerError {
		t.Errorf("GetHTTPStatus(unknown) = %d, want %d", status, http.StatusInternalServerError)
	}
}

// TestNewError tests NewError creates proper AppError
func TestNewError(t *testing.T) {
	tests := []struct {
		name        string
		code        ErrorCode
		wantMessage string
	}{
		{"success", ErrSuccess, "成功"},
		{"not found", ErrNotFound, "资源不存在"},
		{"internal server", ErrInternalServer, "服务器内部错误"},
		{"db connection", ErrDBConnection, "数据库连接失败"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewError(tt.code)
			if err.Code != tt.code {
				t.Errorf("Code = %d, want %d", err.Code, tt.code)
			}
			if err.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", err.Message, tt.wantMessage)
			}
			if err.Detail != "" {
				t.Errorf("Detail should be empty, got %q", err.Detail)
			}
			if err.Err != nil {
				t.Error("Err should be nil")
			}
		})
	}
}

// TestNewError_UnknownCode tests NewError with unknown code uses fallback message
func TestNewError_UnknownCode(t *testing.T) {
	err := NewError(ErrorCode(99999))
	if err.Message != GetErrorMessage(ErrUnknown) {
		t.Errorf("Unknown code should use fallback message, got %q", err.Message)
	}
}

// TestAppError_Unwrap tests the Unwrap method for error chain support
func TestAppError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := NewErrorWithErr(ErrDBQuery, originalErr)

	// Test Unwrap returns original error
	unwrapped := appErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() should return original error")
	}

	// Test errors.Unwrap works
	if errors.Unwrap(appErr) != originalErr {
		t.Error("errors.Unwrap should return original error")
	}

	// Test nil original error
	appErr2 := NewError(ErrNotFound)
	if appErr2.Unwrap() != nil {
		t.Error("Unwrap() should return nil when no original error")
	}
}
