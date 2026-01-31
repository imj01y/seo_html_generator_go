// Package core provides unified response format for HTTP handlers
package core

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Response represents a unified API response structure
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// PagedData represents paginated data
type PagedData struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Pages    int64       `json:"pages"`
}

// NewPagedData creates a new PagedData instance
func NewPagedData(list interface{}, total int64, page, pageSize int) *PagedData {
	pages := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		pages++
	}
	return &PagedData{
		Items:    list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}
}

// getRequestID extracts request ID from gin context
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// Success sends a success response with data
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      int(ErrSuccess),
		Message:   GetErrorMessage(ErrSuccess),
		Data:      data,
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// SuccessWithMessage sends a success response with custom message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      int(ErrSuccess),
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// SuccessPaged sends a success response with paginated data
func SuccessPaged(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:      int(ErrSuccess),
		Message:   GetErrorMessage(ErrSuccess),
		Data:      NewPagedData(list, total, page, pageSize),
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// Fail sends a failure response with default error code
func Fail(c *gin.Context) {
	FailWithCode(c, ErrInternalServer)
}

// FailWithCode sends a failure response with specific error code
func FailWithCode(c *gin.Context, code ErrorCode) {
	httpStatus := GetHTTPStatus(code)
	c.JSON(httpStatus, Response{
		Code:      int(code),
		Message:   GetErrorMessage(code),
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// FailWithMessage sends a failure response with custom message
func FailWithMessage(c *gin.Context, code ErrorCode, message string) {
	httpStatus := GetHTTPStatus(code)
	c.JSON(httpStatus, Response{
		Code:      int(code),
		Message:   message,
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// FailWithError sends a failure response from an AppError
func FailWithError(c *gin.Context, err *AppError) {
	if err == nil {
		Fail(c)
		return
	}

	message := err.Message
	if err.Detail != "" {
		message = err.Message + ": " + err.Detail
	}

	c.JSON(err.HTTPStatus(), Response{
		Code:      int(err.Code),
		Message:   message,
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// HandleError handles an error and sends appropriate response
// It automatically detects AppError and sends proper response
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	// Check if it's an AppError
	if appErr := GetAppError(err); appErr != nil {
		FailWithError(c, appErr)
		return
	}

	// For generic errors, return internal server error
	FailWithMessage(c, ErrInternalServer, err.Error())
}

// FailWithData sends a failure response with error code and additional data
func FailWithData(c *gin.Context, code ErrorCode, data interface{}) {
	httpStatus := GetHTTPStatus(code)
	c.JSON(httpStatus, Response{
		Code:      int(code),
		Message:   GetErrorMessage(code),
		Data:      data,
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// Abort sends a failure response and aborts the request
func Abort(c *gin.Context, code ErrorCode) {
	httpStatus := GetHTTPStatus(code)
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:      int(code),
		Message:   GetErrorMessage(code),
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}

// AbortWithMessage sends a failure response with custom message and aborts
func AbortWithMessage(c *gin.Context, code ErrorCode, message string) {
	httpStatus := GetHTTPStatus(code)
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:      int(code),
		Message:   message,
		Timestamp: time.Now().Unix(),
		RequestID: getRequestID(c),
	})
}
