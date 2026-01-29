# 错误处理与日志实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 统一错误码、响应格式和日志配置，提高系统可维护性和可观测性。

**Architecture:** 错误码体系 + 统一响应格式 + zerolog 结构化日志 + lumberjack 日志轮转

**Tech Stack:** Go, zerolog, lumberjack

---

## Task 1: 定义错误码体系

**Files:**
- Create: `go-page-server/core/errors.go`

**Step 1: 创建错误码定义**

```go
package core

import (
	"fmt"
	"net/http"
)

// ErrorCode 错误码
type ErrorCode int

const (
	// 通用错误 1000-1999
	ErrOK            ErrorCode = 0
	ErrUnknown       ErrorCode = 1000
	ErrInvalidParams ErrorCode = 1001
	ErrUnauthorized  ErrorCode = 1002
	ErrForbidden     ErrorCode = 1003
	ErrNotFound      ErrorCode = 1004
	ErrTimeout       ErrorCode = 1005
	ErrRateLimit     ErrorCode = 1006

	// 数据库错误 2000-2999
	ErrDBConnection ErrorCode = 2000
	ErrDBQuery      ErrorCode = 2001
	ErrDBInsert     ErrorCode = 2002
	ErrDBUpdate     ErrorCode = 2003
	ErrDBDelete     ErrorCode = 2004
	ErrDBNotFound   ErrorCode = 2005

	// 缓存错误 3000-3999
	ErrCacheConnection ErrorCode = 3000
	ErrCacheGet        ErrorCode = 3001
	ErrCacheSet        ErrorCode = 3002
	ErrCacheMiss       ErrorCode = 3003

	// 模板错误 4000-4999
	ErrTemplateNotFound ErrorCode = 4000
	ErrTemplateRender   ErrorCode = 4001
	ErrTemplateCompile  ErrorCode = 4002

	// 池错误 5000-5999
	ErrPoolEmpty    ErrorCode = 5000
	ErrPoolExhausted ErrorCode = 5001

	// 站点错误 6000-6999
	ErrSiteNotFound   ErrorCode = 6000
	ErrSiteDisabled   ErrorCode = 6001
	ErrDomainNotMatch ErrorCode = 6002

	// 调度器错误 7000-7999
	ErrTaskNotFound ErrorCode = 7000
	ErrTaskRunning  ErrorCode = 7001
	ErrInvalidCron  ErrorCode = 7002
)

// 错误码描述
var errorMessages = map[ErrorCode]string{
	ErrOK:            "成功",
	ErrUnknown:       "未知错误",
	ErrInvalidParams: "参数无效",
	ErrUnauthorized:  "未授权",
	ErrForbidden:     "禁止访问",
	ErrNotFound:      "资源不存在",
	ErrTimeout:       "请求超时",
	ErrRateLimit:     "请求过于频繁",

	ErrDBConnection: "数据库连接失败",
	ErrDBQuery:      "数据库查询失败",
	ErrDBInsert:     "数据库插入失败",
	ErrDBUpdate:     "数据库更新失败",
	ErrDBDelete:     "数据库删除失败",
	ErrDBNotFound:   "数据不存在",

	ErrCacheConnection: "缓存连接失败",
	ErrCacheGet:        "缓存读取失败",
	ErrCacheSet:        "缓存写入失败",
	ErrCacheMiss:       "缓存未命中",

	ErrTemplateNotFound: "模板不存在",
	ErrTemplateRender:   "模板渲染失败",
	ErrTemplateCompile:  "模板编译失败",

	ErrPoolEmpty:    "池为空",
	ErrPoolExhausted: "池已耗尽",

	ErrSiteNotFound:   "站点不存在",
	ErrSiteDisabled:   "站点已禁用",
	ErrDomainNotMatch: "域名不匹配",

	ErrTaskNotFound: "任务不存在",
	ErrTaskRunning:  "任务正在运行",
	ErrInvalidCron:  "无效的Cron表达式",
}

// 错误码对应的 HTTP 状态码
var errorHTTPStatus = map[ErrorCode]int{
	ErrOK:            http.StatusOK,
	ErrUnknown:       http.StatusInternalServerError,
	ErrInvalidParams: http.StatusBadRequest,
	ErrUnauthorized:  http.StatusUnauthorized,
	ErrForbidden:     http.StatusForbidden,
	ErrNotFound:      http.StatusNotFound,
	ErrTimeout:       http.StatusGatewayTimeout,
	ErrRateLimit:     http.StatusTooManyRequests,

	ErrDBConnection: http.StatusInternalServerError,
	ErrDBQuery:      http.StatusInternalServerError,
	ErrDBInsert:     http.StatusInternalServerError,
	ErrDBUpdate:     http.StatusInternalServerError,
	ErrDBDelete:     http.StatusInternalServerError,
	ErrDBNotFound:   http.StatusNotFound,

	ErrCacheConnection: http.StatusInternalServerError,
	ErrCacheGet:        http.StatusInternalServerError,
	ErrCacheSet:        http.StatusInternalServerError,
	ErrCacheMiss:       http.StatusOK,

	ErrTemplateNotFound: http.StatusNotFound,
	ErrTemplateRender:   http.StatusInternalServerError,
	ErrTemplateCompile:  http.StatusInternalServerError,

	ErrPoolEmpty:    http.StatusServiceUnavailable,
	ErrPoolExhausted: http.StatusServiceUnavailable,

	ErrSiteNotFound:   http.StatusNotFound,
	ErrSiteDisabled:   http.StatusForbidden,
	ErrDomainNotMatch: http.StatusBadRequest,

	ErrTaskNotFound: http.StatusNotFound,
	ErrTaskRunning:  http.StatusConflict,
	ErrInvalidCron:  http.StatusBadRequest,
}

// AppError 应用错误
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"`
	Err     error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus 返回对应的 HTTP 状态码
func (e *AppError) HTTPStatus() int {
	if status, ok := errorHTTPStatus[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// NewError 创建错误
func NewError(code ErrorCode) *AppError {
	msg := errorMessages[code]
	if msg == "" {
		msg = "未知错误"
	}
	return &AppError{
		Code:    code,
		Message: msg,
	}
}

// NewErrorWithDetail 创建带详情的错误
func NewErrorWithDetail(code ErrorCode, detail string) *AppError {
	err := NewError(code)
	err.Detail = detail
	return err
}

// NewErrorWithErr 创建包装错误
func NewErrorWithErr(code ErrorCode, err error) *AppError {
	appErr := NewError(code)
	appErr.Err = err
	if err != nil {
		appErr.Detail = err.Error()
	}
	return appErr
}

// IsAppError 判断是否为应用错误
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError 获取应用错误
func GetAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return NewErrorWithErr(ErrUnknown, err)
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/errors.go
git commit -m "feat: add error code definitions"
```

---

## Task 2: 统一响应格式

**Files:**
- Create: `go-page-server/core/response.go`

**Step 1: 创建响应结构**

```go
package core

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Response 统一响应格式
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// PagedData 分页数据
type PagedData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Pages    int         `json:"pages"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      int(ErrOK),
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().Unix(),
		RequestID: c.GetString("request_id"),
	})
}

// SuccessWithMessage 成功响应（带消息）
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      int(ErrOK),
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
		RequestID: c.GetString("request_id"),
	})
}

// SuccessPaged 分页成功响应
func SuccessPaged(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	pages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		pages++
	}

	c.JSON(http.StatusOK, Response{
		Code:    int(ErrOK),
		Message: "success",
		Data: PagedData{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			Pages:    pages,
		},
		Timestamp: time.Now().Unix(),
		RequestID: c.GetString("request_id"),
	})
}

// Fail 失败响应
func Fail(c *gin.Context, err *AppError) {
	c.JSON(err.HTTPStatus(), Response{
		Code:      int(err.Code),
		Message:   err.Message,
		Data:      err.Detail,
		Timestamp: time.Now().Unix(),
		RequestID: c.GetString("request_id"),
	})
}

// FailWithCode 使用错误码的失败响应
func FailWithCode(c *gin.Context, code ErrorCode) {
	err := NewError(code)
	Fail(c, err)
}

// FailWithMessage 带消息的失败响应
func FailWithMessage(c *gin.Context, code ErrorCode, message string) {
	err := NewError(code)
	err.Detail = message
	Fail(c, err)
}

// FailWithError 包装错误的失败响应
func FailWithError(c *gin.Context, code ErrorCode, e error) {
	err := NewErrorWithErr(code, e)
	Fail(c, err)
}

// HandleError 自动处理错误
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	if appErr, ok := err.(*AppError); ok {
		Fail(c, appErr)
		return
	}

	Fail(c, NewErrorWithErr(ErrUnknown, err))
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/response.go
git commit -m "feat: add unified response format"
```

---

## Task 3: 配置日志系统

**Files:**
- Create: `go-page-server/core/logger.go`

**Step 1: 创建日志配置**

```go
package core

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // json, console
	Output     string `yaml:"output"`      // stdout, file, both
	FilePath   string `yaml:"file_path"`   // 日志文件路径
	MaxSize    int    `yaml:"max_size"`    // 单文件最大大小 (MB)
	MaxBackups int    `yaml:"max_backups"` // 保留文件数量
	MaxAge     int    `yaml:"max_age"`     // 保留天数
	Compress   bool   `yaml:"compress"`    // 是否压缩
}

// DefaultLogConfig 默认日志配置
func DefaultLogConfig() LogConfig {
	return LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "both",
		FilePath:   "logs/app.log",
		MaxSize:    100,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   true,
	}
}

// SetupLogger 配置日志
func SetupLogger(config LogConfig) {
	// 设置日志级别
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// 设置时间格式
	zerolog.TimeFieldFormat = time.RFC3339

	var writers []io.Writer

	// 配置输出
	switch config.Output {
	case "stdout":
		writers = append(writers, getConsoleWriter(config.Format))
	case "file":
		writers = append(writers, getFileWriter(config))
	case "both":
		writers = append(writers, getConsoleWriter(config.Format))
		writers = append(writers, getFileWriter(config))
	default:
		writers = append(writers, os.Stdout)
	}

	// 创建 multi writer
	multi := zerolog.MultiLevelWriter(writers...)

	// 配置全局 logger
	log.Logger = zerolog.New(multi).
		With().
		Timestamp().
		Caller().
		Logger()
}

// getConsoleWriter 获取控制台输出
func getConsoleWriter(format string) io.Writer {
	if format == "console" {
		return zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		}
	}
	return os.Stdout
}

// getFileWriter 获取文件输出
func getFileWriter(config LogConfig) io.Writer {
	// 确保目录存在
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create log directory")
		return os.Stdout
	}

	return &lumberjack.Logger{
		Filename:   config.FilePath,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
		LocalTime:  true,
	}
}

// RequestLogger 请求日志中间件
func RequestLogger() func(c *gin.Context) {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 生成请求 ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// 处理请求
		c.Next()

		// 记录日志
		latency := time.Since(start)
		status := c.Writer.Status()

		event := log.Info()
		if status >= 500 {
			event = log.Error()
		} else if status >= 400 {
			event = log.Warn()
		}

		event.
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Int("status", status).
			Dur("latency", latency).
			Str("client_ip", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent()).
			Int("body_size", c.Writer.Size()).
			Msg("HTTP Request")
	}
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}
```

需要添加导入：

```go
import (
	"fmt"
	"math/rand"

	"github.com/gin-gonic/gin"
)
```

**Step 2: Commit**

```bash
git add go-page-server/core/logger.go
git commit -m "feat: add structured logging with zerolog"
```

---

## Task 4: 添加恢复中间件

**Files:**
- Create: `go-page-server/middleware/recovery.go`

**Step 1: 创建恢复中间件**

```go
package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"your-module/core"
)

// Recovery panic 恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录堆栈
				stack := string(debug.Stack())

				log.Error().
					Str("request_id", c.GetString("request_id")).
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Interface("error", err).
					Str("stack", stack).
					Msg("Panic recovered")

				// 返回 500 错误
				c.AbortWithStatusJSON(http.StatusInternalServerError, core.Response{
					Code:      int(core.ErrUnknown),
					Message:   "服务器内部错误",
					RequestID: c.GetString("request_id"),
				})
			}
		}()

		c.Next()
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/middleware/recovery.go
git commit -m "feat: add panic recovery middleware"
```

---

## Task 5: 添加测试

**Files:**
- Create: `go-page-server/core/errors_test.go`

**Step 1: 创建测试文件**

```go
package core

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := NewError(ErrNotFound)
	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{ErrOK, http.StatusOK},
		{ErrNotFound, http.StatusNotFound},
		{ErrUnauthorized, http.StatusUnauthorized},
		{ErrInvalidParams, http.StatusBadRequest},
		{ErrDBConnection, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		err := NewError(tt.code)
		if err.HTTPStatus() != tt.expected {
			t.Errorf("Code %d: expected status %d, got %d",
				tt.code, tt.expected, err.HTTPStatus())
		}
	}
}

func TestNewErrorWithDetail(t *testing.T) {
	detail := "详细错误信息"
	err := NewErrorWithDetail(ErrInvalidParams, detail)

	if err.Detail != detail {
		t.Errorf("Expected detail %s, got %s", detail, err.Detail)
	}
}

func TestNewErrorWithErr(t *testing.T) {
	originalErr := errors.New("original error")
	err := NewErrorWithErr(ErrDBQuery, originalErr)

	if err.Err != originalErr {
		t.Error("Expected wrapped error")
	}

	if err.Detail != originalErr.Error() {
		t.Errorf("Expected detail %s, got %s", originalErr.Error(), err.Detail)
	}
}

func TestIsAppError(t *testing.T) {
	appErr := NewError(ErrNotFound)
	normalErr := errors.New("normal error")

	if !IsAppError(appErr) {
		t.Error("Expected true for AppError")
	}

	if IsAppError(normalErr) {
		t.Error("Expected false for normal error")
	}
}

func TestGetAppError(t *testing.T) {
	appErr := NewError(ErrNotFound)
	normalErr := errors.New("normal error")

	result := GetAppError(appErr)
	if result.Code != ErrNotFound {
		t.Errorf("Expected code %d, got %d", ErrNotFound, result.Code)
	}

	result = GetAppError(normalErr)
	if result.Code != ErrUnknown {
		t.Errorf("Expected code %d, got %d", ErrUnknown, result.Code)
	}
}

func TestErrorMessages(t *testing.T) {
	// 确保所有错误码都有消息
	codes := []ErrorCode{
		ErrOK, ErrUnknown, ErrInvalidParams, ErrUnauthorized,
		ErrForbidden, ErrNotFound, ErrTimeout, ErrRateLimit,
		ErrDBConnection, ErrDBQuery, ErrDBInsert, ErrDBUpdate,
		ErrDBDelete, ErrDBNotFound,
		ErrCacheConnection, ErrCacheGet, ErrCacheSet, ErrCacheMiss,
		ErrTemplateNotFound, ErrTemplateRender, ErrTemplateCompile,
		ErrPoolEmpty, ErrPoolExhausted,
		ErrSiteNotFound, ErrSiteDisabled, ErrDomainNotMatch,
		ErrTaskNotFound, ErrTaskRunning, ErrInvalidCron,
	}

	for _, code := range codes {
		msg := errorMessages[code]
		if msg == "" {
			t.Errorf("Missing message for error code %d", code)
		}
	}
}
```

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./core/... -run TestAppError
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/core/errors_test.go
git commit -m "test: add error handling tests"
```

---

## Task 6: 更新 main.go 初始化

**Files:**
- Modify: `go-page-server/main.go`

**Step 1: 添加日志和中间件初始化**

在 `main()` 函数开头添加：

```go
// 配置日志
logConfig := core.DefaultLogConfig()
if cfg.Log != nil {
	logConfig = *cfg.Log
}
core.SetupLogger(logConfig)

// 创建 Gin 引擎
gin.SetMode(gin.ReleaseMode)
r := gin.New()

// 添加中间件
r.Use(core.RequestLogger())
r.Use(middleware.Recovery())
```

**Step 2: Commit**

```bash
git add go-page-server/main.go
git commit -m "feat: initialize logger and middleware in main"
```

---

## 完成检查清单

- [ ] Task 1: 错误码定义
- [ ] Task 2: 响应格式
- [ ] Task 3: 日志配置
- [ ] Task 4: 恢复中间件
- [ ] Task 5: 测试覆盖
- [ ] Task 6: main.go 初始化

所有任务完成后运行完整测试：

```bash
cd go-page-server && go test -v ./...
```