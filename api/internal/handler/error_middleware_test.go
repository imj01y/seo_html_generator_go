package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	core "seo-generator/api/internal/service"
)

func TestErrorMiddleware_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   core.ErrorCode
	}{
		{
			name:           "ValidationError returns 400",
			err:            core.NewError(core.ErrValidation),
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   core.ErrValidation,
		},
		{
			name:           "NotFoundError returns 404",
			err:            core.NewError(core.ErrNotFound),
			expectedStatus: http.StatusNotFound,
			expectedCode:   core.ErrNotFound,
		},
		{
			name:           "DatabaseError returns 500",
			err:            core.NewError(core.ErrDBQuery),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   core.ErrDBQuery,
		},
		{
			name:           "PoolExhaustedError returns 503",
			err:            core.NewError(core.ErrPoolExhausted),
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   core.ErrPoolExhausted,
		},
		{
			name:           "UnauthorizedError returns 401",
			err:            core.NewError(core.ErrUnauthorized),
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   core.ErrUnauthorized,
		},
		{
			name:           "ForbiddenError returns 403",
			err:            core.NewError(core.ErrForbidden),
			expectedStatus: http.StatusForbidden,
			expectedCode:   core.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// 模拟 handler 添加错误
			c.Error(tt.err)

			// 执行中间件
			ErrorHandlerMiddleware()(c)

			// 验证状态码
			assert.Equal(t, tt.expectedStatus, w.Code)

			// 验证响应格式
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// 验证错误代码
			code, ok := response["code"].(float64)
			assert.True(t, ok)
			assert.Equal(t, int(tt.expectedCode), int(code))

			// 验证消息存在
			message, ok := response["message"].(string)
			assert.True(t, ok)
			assert.NotEmpty(t, message)
		})
	}
}

func TestErrorMiddleware_WithDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 创建带详情的错误
	err := core.NewErrorWithDetail(core.ErrValidation, "email field is required")
	c.Error(err)

	ErrorHandlerMiddleware()(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, float64(core.ErrValidation), response["code"])
	assert.NotEmpty(t, response["message"])
	assert.Equal(t, "email field is required", response["detail"])
}

func TestErrorMiddleware_GenericError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 添加普通错误
	c.Error(errors.New("something went wrong"))

	ErrorHandlerMiddleware()(c)

	// 验证返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// 验证响应格式
	code, ok := response["code"].(float64)
	assert.True(t, ok)
	assert.Equal(t, int(core.ErrInternalServer), int(code))

	message, ok := response["message"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, message)
}

func TestErrorMiddleware_NoError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 不添加任何错误
	ErrorHandlerMiddleware()(c)

	// 中间件不应该设置任何响应
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestErrorMiddleware_MultipleErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 添加多个错误
	c.Error(core.NewError(core.ErrValidation))
	c.Error(core.NewError(core.ErrNotFound))
	c.Error(core.NewError(core.ErrUnauthorized))

	ErrorHandlerMiddleware()(c)

	// 应该处理最后一个错误
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, float64(core.ErrUnauthorized), response["code"])
}

func TestErrorMiddleware_WrappedError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 创建包装的错误
	originalErr := errors.New("database connection failed")
	appErr := core.NewErrorWithErr(core.ErrDBConnection, originalErr)
	c.Error(appErr)

	ErrorHandlerMiddleware()(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, float64(core.ErrDBConnection), response["code"])
	assert.NotEmpty(t, response["message"])
	assert.Equal(t, "database connection failed", response["detail"])
}
