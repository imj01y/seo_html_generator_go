// Package core provides error code definitions and error handling utilities
package core

import (
	"fmt"
	"net/http"
)

// ErrorCode represents an error code type
type ErrorCode int

// Error code constants
const (
	// Common errors (1000-1999)
	ErrSuccess         ErrorCode = 0
	ErrUnknown         ErrorCode = 1000
	ErrInvalidParam    ErrorCode = 1001
	ErrUnauthorized    ErrorCode = 1002
	ErrForbidden       ErrorCode = 1003
	ErrNotFound        ErrorCode = 1004
	ErrMethodNotAllow  ErrorCode = 1005
	ErrTooManyRequests ErrorCode = 1006
	ErrInternalServer  ErrorCode = 1007
	ErrTimeout         ErrorCode = 1008
	ErrValidation      ErrorCode = 1009

	// Database errors (2000-2999)
	ErrDBConnection ErrorCode = 2000
	ErrDBQuery      ErrorCode = 2001
	ErrDBInsert     ErrorCode = 2002
	ErrDBUpdate     ErrorCode = 2003
	ErrDBDelete     ErrorCode = 2004
	ErrDBDuplicate  ErrorCode = 2005
	ErrDBNotFound   ErrorCode = 2006
	ErrDBTxBegin    ErrorCode = 2007
	ErrDBTxCommit   ErrorCode = 2008
	ErrDBTxRollback ErrorCode = 2009

	// Cache errors (3000-3999)
	ErrCacheConnection ErrorCode = 3000
	ErrCacheGet        ErrorCode = 3001
	ErrCacheSet        ErrorCode = 3002
	ErrCacheDelete     ErrorCode = 3003
	ErrCacheExpired    ErrorCode = 3004
	ErrCacheFull       ErrorCode = 3005
	ErrCacheMiss       ErrorCode = 3006
	ErrCacheInvalid    ErrorCode = 3007

	// Template errors (4000-4999)
	ErrTemplateNotFound  ErrorCode = 4000
	ErrTemplateParse     ErrorCode = 4001
	ErrTemplateRender    ErrorCode = 4002
	ErrTemplateInvalid   ErrorCode = 4003
	ErrTemplateCompile   ErrorCode = 4004
	ErrTemplateSyntax    ErrorCode = 4005
	ErrTemplateExecution ErrorCode = 4006
	ErrTemplateDataType  ErrorCode = 4007

	// Pool errors (5000-5999)
	ErrPoolExhausted ErrorCode = 5000
	ErrPoolTimeout   ErrorCode = 5001
	ErrPoolClosed    ErrorCode = 5002
	ErrPoolInvalid   ErrorCode = 5003
	ErrPoolOverflow  ErrorCode = 5004
	ErrPoolGetFailed ErrorCode = 5005
	ErrPoolPutFailed ErrorCode = 5006

	// Site errors (6000-6999)
	ErrSiteNotFound   ErrorCode = 6000
	ErrSiteDisabled   ErrorCode = 6001
	ErrSiteInvalid    ErrorCode = 6002
	ErrSiteDomain     ErrorCode = 6003
	ErrSiteConfig     ErrorCode = 6004
	ErrSiteTemplate   ErrorCode = 6005
	ErrSiteGroup      ErrorCode = 6006
	ErrSitePermission ErrorCode = 6007

	// Scheduler errors (7000-7999)
	ErrSchedulerNotRunning   ErrorCode = 7000
	ErrSchedulerTaskExist    ErrorCode = 7001
	ErrSchedulerTaskNotFound ErrorCode = 7002
	ErrSchedulerInvalidCron  ErrorCode = 7003
	ErrSchedulerExecFailed   ErrorCode = 7004
	ErrSchedulerQueueFull    ErrorCode = 7005
	ErrSchedulerTimeout      ErrorCode = 7006
	ErrSchedulerCancelled    ErrorCode = 7007
)

// errorMessages maps error codes to human-readable messages
var errorMessages = map[ErrorCode]string{
	// Common errors
	ErrSuccess:         "成功",
	ErrUnknown:         "未知错误",
	ErrInvalidParam:    "参数无效",
	ErrUnauthorized:    "未授权",
	ErrForbidden:       "禁止访问",
	ErrNotFound:        "资源不存在",
	ErrMethodNotAllow:  "方法不允许",
	ErrTooManyRequests: "请求过于频繁",
	ErrInternalServer:  "服务器内部错误",
	ErrTimeout:         "请求超时",
	ErrValidation:      "数据验证失败",

	// Database errors
	ErrDBConnection: "数据库连接失败",
	ErrDBQuery:      "数据库查询失败",
	ErrDBInsert:     "数据库插入失败",
	ErrDBUpdate:     "数据库更新失败",
	ErrDBDelete:     "数据库删除失败",
	ErrDBDuplicate:  "数据重复",
	ErrDBNotFound:   "数据不存在",
	ErrDBTxBegin:    "事务开启失败",
	ErrDBTxCommit:   "事务提交失败",
	ErrDBTxRollback: "事务回滚失败",

	// Cache errors
	ErrCacheConnection: "缓存连接失败",
	ErrCacheGet:        "缓存读取失败",
	ErrCacheSet:        "缓存写入失败",
	ErrCacheDelete:     "缓存删除失败",
	ErrCacheExpired:    "缓存已过期",
	ErrCacheFull:       "缓存已满",
	ErrCacheMiss:       "缓存未命中",
	ErrCacheInvalid:    "缓存数据无效",

	// Template errors
	ErrTemplateNotFound:  "模板不存在",
	ErrTemplateParse:     "模板解析失败",
	ErrTemplateRender:    "模板渲染失败",
	ErrTemplateInvalid:   "模板格式无效",
	ErrTemplateCompile:   "模板编译失败",
	ErrTemplateSyntax:    "模板语法错误",
	ErrTemplateExecution: "模板执行失败",
	ErrTemplateDataType:  "模板数据类型错误",

	// Pool errors
	ErrPoolExhausted: "对象池已耗尽",
	ErrPoolTimeout:   "对象池获取超时",
	ErrPoolClosed:    "对象池已关闭",
	ErrPoolInvalid:   "对象池无效",
	ErrPoolOverflow:  "对象池溢出",
	ErrPoolGetFailed: "对象池获取失败",
	ErrPoolPutFailed: "对象池归还失败",

	// Site errors
	ErrSiteNotFound:   "站点不存在",
	ErrSiteDisabled:   "站点已禁用",
	ErrSiteInvalid:    "站点配置无效",
	ErrSiteDomain:     "站点域名错误",
	ErrSiteConfig:     "站点配置错误",
	ErrSiteTemplate:   "站点模板错误",
	ErrSiteGroup:      "站点分组错误",
	ErrSitePermission: "站点权限不足",

	// Scheduler errors
	ErrSchedulerNotRunning:   "调度器未运行",
	ErrSchedulerTaskExist:    "任务已存在",
	ErrSchedulerTaskNotFound: "任务不存在",
	ErrSchedulerInvalidCron:  "无效的Cron表达式",
	ErrSchedulerExecFailed:   "任务执行失败",
	ErrSchedulerQueueFull:    "任务队列已满",
	ErrSchedulerTimeout:      "任务执行超时",
	ErrSchedulerCancelled:    "任务已取消",
}

// errorHTTPStatus maps error codes to HTTP status codes
var errorHTTPStatus = map[ErrorCode]int{
	// Common errors
	ErrSuccess:         http.StatusOK,
	ErrUnknown:         http.StatusInternalServerError,
	ErrInvalidParam:    http.StatusBadRequest,
	ErrUnauthorized:    http.StatusUnauthorized,
	ErrForbidden:       http.StatusForbidden,
	ErrNotFound:        http.StatusNotFound,
	ErrMethodNotAllow:  http.StatusMethodNotAllowed,
	ErrTooManyRequests: http.StatusTooManyRequests,
	ErrInternalServer:  http.StatusInternalServerError,
	ErrTimeout:         http.StatusGatewayTimeout,
	ErrValidation:      http.StatusUnprocessableEntity,

	// Database errors
	ErrDBConnection: http.StatusServiceUnavailable,
	ErrDBQuery:      http.StatusInternalServerError,
	ErrDBInsert:     http.StatusInternalServerError,
	ErrDBUpdate:     http.StatusInternalServerError,
	ErrDBDelete:     http.StatusInternalServerError,
	ErrDBDuplicate:  http.StatusConflict,
	ErrDBNotFound:   http.StatusNotFound,
	ErrDBTxBegin:    http.StatusInternalServerError,
	ErrDBTxCommit:   http.StatusInternalServerError,
	ErrDBTxRollback: http.StatusInternalServerError,

	// Cache errors
	ErrCacheConnection: http.StatusServiceUnavailable,
	ErrCacheGet:        http.StatusInternalServerError,
	ErrCacheSet:        http.StatusInternalServerError,
	ErrCacheDelete:     http.StatusInternalServerError,
	ErrCacheExpired:    http.StatusGone,
	ErrCacheFull:       http.StatusInsufficientStorage,
	ErrCacheMiss:       http.StatusNotFound,
	ErrCacheInvalid:    http.StatusInternalServerError,

	// Template errors
	ErrTemplateNotFound:  http.StatusNotFound,
	ErrTemplateParse:     http.StatusInternalServerError,
	ErrTemplateRender:    http.StatusInternalServerError,
	ErrTemplateInvalid:   http.StatusBadRequest,
	ErrTemplateCompile:   http.StatusInternalServerError,
	ErrTemplateSyntax:    http.StatusBadRequest,
	ErrTemplateExecution: http.StatusInternalServerError,
	ErrTemplateDataType:  http.StatusBadRequest,

	// Pool errors
	ErrPoolExhausted: http.StatusServiceUnavailable,
	ErrPoolTimeout:   http.StatusServiceUnavailable,
	ErrPoolClosed:    http.StatusServiceUnavailable,
	ErrPoolInvalid:   http.StatusInternalServerError,
	ErrPoolOverflow:  http.StatusServiceUnavailable,
	ErrPoolGetFailed: http.StatusInternalServerError,
	ErrPoolPutFailed: http.StatusInternalServerError,

	// Site errors
	ErrSiteNotFound:   http.StatusNotFound,
	ErrSiteDisabled:   http.StatusForbidden,
	ErrSiteInvalid:    http.StatusBadRequest,
	ErrSiteDomain:     http.StatusBadRequest,
	ErrSiteConfig:     http.StatusInternalServerError,
	ErrSiteTemplate:   http.StatusInternalServerError,
	ErrSiteGroup:      http.StatusBadRequest,
	ErrSitePermission: http.StatusForbidden,

	// Scheduler errors
	ErrSchedulerNotRunning:   http.StatusServiceUnavailable,
	ErrSchedulerTaskExist:    http.StatusConflict,
	ErrSchedulerTaskNotFound: http.StatusNotFound,
	ErrSchedulerInvalidCron:  http.StatusBadRequest,
	ErrSchedulerExecFailed:   http.StatusInternalServerError,
	ErrSchedulerQueueFull:    http.StatusServiceUnavailable,
	ErrSchedulerTimeout:      http.StatusGatewayTimeout,
	ErrSchedulerCancelled:    http.StatusRequestTimeout,
}

// AppError represents an application error with code and message
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"`
	Err     error     `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus returns the HTTP status code for this error
func (e *AppError) HTTPStatus() int {
	if status, ok := errorHTTPStatus[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// NewError creates a new AppError with the given error code
func NewError(code ErrorCode) *AppError {
	msg := errorMessages[code]
	if msg == "" {
		msg = errorMessages[ErrUnknown]
	}
	return &AppError{
		Code:    code,
		Message: msg,
	}
}

// NewErrorWithDetail creates a new AppError with code and detail message
func NewErrorWithDetail(code ErrorCode, detail string) *AppError {
	err := NewError(code)
	err.Detail = detail
	return err
}

// NewErrorWithErr creates a new AppError wrapping an existing error
func NewErrorWithErr(code ErrorCode, err error) *AppError {
	appErr := NewError(code)
	if err != nil {
		appErr.Err = err
		appErr.Detail = err.Error()
	}
	return appErr
}

// IsAppError checks if the given error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError extracts AppError from an error, returns nil if not an AppError
func GetAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return nil
}

// GetErrorMessage returns the message for an error code
func GetErrorMessage(code ErrorCode) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return errorMessages[ErrUnknown]
}

// GetHTTPStatus returns the HTTP status code for an error code
func GetHTTPStatus(code ErrorCode) int {
	if status, ok := errorHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}
