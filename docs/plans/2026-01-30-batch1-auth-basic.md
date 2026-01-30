# Batch 1: 认证与基础模块实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现认证模块（4个接口）和基础日志/仪表盘模块（6个接口），为后续 CRUD 模块奠定基础

**Architecture:** Go Gin 框架 + JWT 认证 + bcrypt 密码哈希，沿用已有的 response.go 响应格式，添加兼容 Python API 的响应包装

**Tech Stack:** Go 1.22+, Gin, golang-jwt/jwt, golang.org/x/crypto/bcrypt, MySQL

---

## 前置条件

- go-page-server 已有代码保持不变
- 数据库表 `admins` 已存在（见 schema.sql）
- 配置文件 `config.yaml` 包含 auth 配置

---

## Task 1: 添加 JWT 和 bcrypt 依赖

**Files:**
- Modify: `go-page-server/go.mod`

**Step 1: 添加依赖**

```bash
cd go-page-server
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
```

**Step 2: 验证依赖已添加**

Run: `go mod tidy && cat go.mod | grep -E "jwt|bcrypt"`
Expected: 显示两个依赖

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: 添加 JWT 和 bcrypt 依赖"
```

---

## Task 2: 创建认证核心模块

**Files:**
- Create: `go-page-server/core/auth.go`
- Test: `go-page-server/core/auth_test.go`

**Step 1: 写测试文件**

```go
// go-page-server/core/auth_test.go
package core

import (
	"testing"
	"time"
)

func TestHashPassword(t *testing.T) {
	password := "test123456"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("Hash should not be empty")
	}
	if hash == password {
		t.Fatal("Hash should not equal plain password")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "test123456"
	hash, _ := HashPassword(password)

	// 正确密码
	if !VerifyPassword(password, hash) {
		t.Fatal("VerifyPassword should return true for correct password")
	}

	// 错误密码
	if VerifyPassword("wrongpassword", hash) {
		t.Fatal("VerifyPassword should return false for wrong password")
	}
}

func TestCreateAndVerifyToken(t *testing.T) {
	// 设置测试用的密钥
	testSecret := "test-secret-key-for-unit-test"

	claims := map[string]interface{}{
		"sub":      "admin",
		"admin_id": 1,
		"role":     "admin",
	}

	token, err := CreateAccessToken(claims, testSecret, 60*time.Minute)
	if err != nil {
		t.Fatalf("CreateAccessToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("Token should not be empty")
	}

	// 验证 token
	parsed, err := VerifyToken(token, testSecret)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if parsed["sub"] != "admin" {
		t.Fatalf("Expected sub=admin, got %v", parsed["sub"])
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	testSecret := "test-secret-key"
	claims := map[string]interface{}{"sub": "admin"}

	// 创建已过期的 token（-1分钟）
	token, _ := CreateAccessToken(claims, testSecret, -1*time.Minute)

	_, err := VerifyToken(token, testSecret)
	if err == nil {
		t.Fatal("VerifyToken should fail for expired token")
	}
}
```

**Step 2: 运行测试，确认失败**

Run: `cd go-page-server && go test ./core -run "TestHash|TestVerify|TestCreate" -v`
Expected: FAIL (函数未定义)

**Step 3: 实现 auth.go**

```go
// go-page-server/core/auth.go
package core

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrTokenExpired  = errors.New("token expired")
	ErrInvalidClaims = errors.New("invalid claims")
)

// HashPassword 使用 bcrypt 哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// VerifyPassword 验证密码是否匹配
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CreateAccessToken 创建 JWT Token
func CreateAccessToken(claims map[string]interface{}, secret string, expiry time.Duration) (string, error) {
	now := time.Now()

	// 构建 JWT claims
	jwtClaims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(expiry).Unix(),
	}

	// 添加自定义 claims
	for k, v := range claims {
		jwtClaims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	return token.SignedString([]byte(secret))
}

// VerifyToken 验证 JWT Token 并返回 claims
func VerifyToken(tokenString, secret string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		result := make(map[string]interface{})
		for k, v := range claims {
			result[k] = v
		}
		return result, nil
	}

	return nil, ErrInvalidClaims
}
```

**Step 4: 运行测试，确认通过**

Run: `cd go-page-server && go test ./core -run "TestHash|TestVerify|TestCreate" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go-page-server/core/auth.go go-page-server/core/auth_test.go
git commit -m "feat(core): 添加认证核心模块 - 密码哈希和 JWT"
```

---

## Task 3: 添加认证配置

**Files:**
- Modify: `go-page-server/config/config.go`

**Step 1: 添加 AuthConfig 结构**

在 `config.go` 的 Config 结构中添加：

```go
// AuthConfig holds authentication configuration
type AuthConfig struct {
	SecretKey              string `yaml:"secret_key"`
	Algorithm              string `yaml:"algorithm"`
	AccessTokenExpireMinutes int  `yaml:"access_token_expire_minutes"`
	DefaultAdmin           struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"default_admin"`
}

// 在 Config 结构中添加
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Database       DatabaseConfig       `yaml:"database"`
	Cache          CacheConfig          `yaml:"cache"`
	SpiderDetector SpiderDetectorConfig `yaml:"spider_detector"`
	Auth           AuthConfig           `yaml:"auth"`  // 新增
}
```

**Step 2: 在 Load 函数中解析 auth 配置**

在 `Load` 函数的 cfg 初始化中添加：

```go
Auth: AuthConfig{
	SecretKey:              getString(merged, "auth.secret_key", "default-secret-key-change-in-production"),
	Algorithm:              getString(merged, "auth.algorithm", "HS256"),
	AccessTokenExpireMinutes: getInt(merged, "auth.access_token_expire_minutes", 1440),
},
```

**Step 3: 验证编译通过**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add go-page-server/config/config.go
git commit -m "feat(config): 添加认证配置 AuthConfig"
```

---

## Task 4: 创建认证中间件

**Files:**
- Create: `go-page-server/api/middleware.go`
- Test: `go-page-server/api/middleware_test.go`

**Step 1: 写测试文件**

```go
// go-page-server/api/middleware_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-page-server/core"
)

func TestAuthMiddleware_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware("test-secret"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"

	r := gin.New()
	r.Use(AuthMiddleware(secret))
	r.GET("/test", func(c *gin.Context) {
		claims, _ := c.Get("claims")
		c.JSON(200, claims)
	})

	// 创建有效 token
	token, _ := core.CreateAccessToken(map[string]interface{}{
		"sub":      "admin",
		"admin_id": 1,
	}, secret, time.Hour)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"

	r := gin.New()
	r.Use(AuthMiddleware(secret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 创建过期 token
	token, _ := core.CreateAccessToken(map[string]interface{}{
		"sub": "admin",
	}, secret, -time.Hour)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}
```

**Step 2: 运行测试，确认失败**

Run: `cd go-page-server && go test ./api -run TestAuthMiddleware -v`
Expected: FAIL

**Step 3: 实现 middleware.go**

```go
// go-page-server/api/middleware.go
package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	"go-page-server/core"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取 Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			core.AbortWithMessage(c, core.ErrUnauthorized, "缺少认证信息")
			return
		}

		// 解析 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			core.AbortWithMessage(c, core.ErrUnauthorized, "认证格式错误")
			return
		}

		token := parts[1]

		// 验证 token
		claims, err := core.VerifyToken(token, secret)
		if err != nil {
			if err == core.ErrTokenExpired {
				core.AbortWithMessage(c, core.ErrUnauthorized, "Token 已过期")
			} else {
				core.AbortWithMessage(c, core.ErrUnauthorized, "无效的 Token")
			}
			return
		}

		// 将 claims 存入 context
		c.Set("claims", claims)
		c.Set("admin_id", claims["admin_id"])
		c.Set("username", claims["sub"])

		c.Next()
	}
}
```

**Step 4: 运行测试，确认通过**

Run: `cd go-page-server && go test ./api -run TestAuthMiddleware -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go-page-server/api/middleware.go go-page-server/api/middleware_test.go
git commit -m "feat(api): 添加 JWT 认证中间件"
```

---

## Task 5: 实现认证 API handlers

**Files:**
- Create: `go-page-server/api/auth.go`
- Test: `go-page-server/api/auth_test.go`

**Step 1: 写测试文件**

```go
// go-page-server/api/auth_test.go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoginHandler_Success(t *testing.T) {
	// 此测试需要数据库，在集成测试中运行
	t.Skip("Integration test - requires database")
}

func TestLoginHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authHandler := NewAuthHandler("test-secret", 1440, nil)
	r.POST("/api/auth/login", authHandler.Login)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestLogoutHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authHandler := NewAuthHandler("test-secret", 1440, nil)
	r.POST("/api/auth/logout", authHandler.Logout)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Fatalf("Expected code 0, got %v", resp["code"])
	}
}
```

**Step 2: 运行测试，确认失败**

Run: `cd go-page-server && go test ./api -run "TestLogin|TestLogout" -v`
Expected: FAIL

**Step 3: 实现 auth.go**

```go
// go-page-server/api/auth.go
package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// AuthHandler 认证相关 handler
type AuthHandler struct {
	db           *sqlx.DB
	secret       string
	expireMinutes int
}

// NewAuthHandler 创建 AuthHandler
func NewAuthHandler(secret string, expireMinutes int, db *sqlx.DB) *AuthHandler {
	return &AuthHandler{
		db:           db,
		secret:       secret,
		expireMinutes: expireMinutes,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应数据
type LoginResponse struct {
	Token   string `json:"token,omitempty"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Login 管理员登录
// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrBadRequest, "请求参数错误")
		return
	}

	// 查询管理员
	var admin struct {
		ID        int       `db:"id"`
		Username  string    `db:"username"`
		Password  string    `db:"password"`
		LastLogin *time.Time `db:"last_login"`
	}

	err := h.db.Get(&admin, "SELECT id, username, password, last_login FROM admins WHERE username = ?", req.Username)
	if err != nil {
		log.Debug().Str("username", req.Username).Msg("Admin not found")
		core.Success(c, LoginResponse{
			Success: false,
			Message: "用户名或密码错误",
		})
		return
	}

	// 验证密码
	if !core.VerifyPassword(req.Password, admin.Password) {
		log.Debug().Str("username", req.Username).Msg("Invalid password")
		core.Success(c, LoginResponse{
			Success: false,
			Message: "用户名或密码错误",
		})
		return
	}

	// 更新最后登录时间
	_, err = h.db.Exec("UPDATE admins SET last_login = NOW() WHERE id = ?", admin.ID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to update last login time")
	}

	// 创建 token
	token, err := core.CreateAccessToken(map[string]interface{}{
		"sub":      admin.Username,
		"admin_id": admin.ID,
		"role":     "admin",
	}, h.secret, time.Duration(h.expireMinutes)*time.Minute)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create token")
		core.FailWithMessage(c, core.ErrInternalServer, "Token 生成失败")
		return
	}

	core.Success(c, LoginResponse{
		Success: true,
		Token:   token,
		Message: "登录成功",
	})
}

// Logout 退出登录
// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// JWT 是无状态的，客户端删除 token 即可
	core.Success(c, gin.H{"success": true})
}

// Profile 获取当前用户信息
// GET /api/auth/profile
func (h *AuthHandler) Profile(c *gin.Context) {
	claims, exists := c.Get("claims")
	if !exists {
		core.FailWithCode(c, core.ErrUnauthorized)
		return
	}

	claimsMap := claims.(map[string]interface{})
	username := claimsMap["sub"].(string)

	// 查询最新用户信息
	var admin struct {
		ID        int        `db:"id"`
		Username  string     `db:"username"`
		LastLogin *time.Time `db:"last_login"`
	}

	err := h.db.Get(&admin, "SELECT id, username, last_login FROM admins WHERE username = ?", username)
	if err != nil {
		core.Success(c, gin.H{
			"username":   username,
			"role":       "admin",
			"last_login": nil,
		})
		return
	}

	var lastLogin interface{}
	if admin.LastLogin != nil {
		lastLogin = admin.LastLogin.Format(time.RFC3339)
	}

	core.Success(c, gin.H{
		"id":         admin.ID,
		"username":   admin.Username,
		"role":       "admin",
		"last_login": lastLogin,
	})
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword 修改密码
// POST /api/auth/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrBadRequest, "请求参数错误")
		return
	}

	claims, _ := c.Get("claims")
	claimsMap := claims.(map[string]interface{})
	username := claimsMap["sub"].(string)
	adminID := int(claimsMap["admin_id"].(float64))

	// 验证旧密码
	var storedPassword string
	err := h.db.Get(&storedPassword, "SELECT password FROM admins WHERE username = ?", username)
	if err != nil {
		core.Success(c, gin.H{"success": false, "message": "用户不存在"})
		return
	}

	if !core.VerifyPassword(req.OldPassword, storedPassword) {
		core.Success(c, gin.H{"success": false, "message": "旧密码错误"})
		return
	}

	// 哈希新密码
	newHash, err := core.HashPassword(req.NewPassword)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "密码加密失败")
		return
	}

	// 更新密码
	_, err = h.db.Exec("UPDATE admins SET password = ? WHERE id = ?", newHash, adminID)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "密码更新失败")
		return
	}

	core.Success(c, gin.H{"success": true, "message": "密码修改成功"})
}
```

**Step 4: 运行测试，确认通过**

Run: `cd go-page-server && go test ./api -run "TestLogin|TestLogout" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go-page-server/api/auth.go go-page-server/api/auth_test.go
git commit -m "feat(api): 实现认证 API - login/logout/profile/change-password"
```

---

## Task 6: 注册认证路由

**Files:**
- Modify: `go-page-server/api/router.go`

**Step 1: 在 SetupRouter 中添加认证路由**

在 `router.go` 的 `SetupRouter` 函数中添加：

```go
// 认证相关路由（无需认证）
authHandler := NewAuthHandler(
	cfg.Auth.SecretKey,
	cfg.Auth.AccessTokenExpireMinutes,
	deps.DB,
)

authGroup := r.Group("/api/auth")
{
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/logout", authHandler.Logout)
}

// 需要认证的路由
authRequired := r.Group("/api/auth")
authRequired.Use(AuthMiddleware(cfg.Auth.SecretKey))
{
	authRequired.GET("/profile", authHandler.Profile)
	authRequired.POST("/change-password", authHandler.ChangePassword)
}
```

**Step 2: 更新 Dependencies 结构**

在 `router.go` 的 `Dependencies` 结构中添加：

```go
type Dependencies struct {
	// ... 已有字段
	DB     *sqlx.DB
	Config *config.Config
}
```

**Step 3: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add go-page-server/api/router.go
git commit -m "feat(api): 注册认证路由到 router"
```

---

## Task 7: 更新 main.go 传递依赖

**Files:**
- Modify: `go-page-server/main.go`

**Step 1: 在 Dependencies 中添加 DB 和 Config**

在 `main.go` 中更新 deps 初始化：

```go
deps := &api.Dependencies{
	TemplateAnalyzer: templateAnalyzer,
	TemplateFuncs:    funcsManager,
	DataPoolManager:  dataPoolManager,
	Scheduler:        scheduler,
	TemplateCache:    templateCache,
	Monitor:          monitor,
	DB:               db,      // 添加
	Config:           cfg,     // 添加
}
```

**Step 2: 验证编译和启动**

Run: `cd go-page-server && go build -o server.exe . && echo "Build OK"`
Expected: Build OK

**Step 3: Commit**

```bash
git add go-page-server/main.go
git commit -m "feat(main): 传递 DB 和 Config 到 API Dependencies"
```

---

## Task 8: 添加仪表盘 API

**Files:**
- Create: `go-page-server/api/dashboard.go`
- Test: `go-page-server/api/dashboard_test.go`

**Step 1: 写测试**

```go
// go-page-server/api/dashboard_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDashboardStats_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware("test-secret"))

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/stats", handler.Stats)

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}
```

**Step 2: 运行测试确认失败**

Run: `cd go-page-server && go test ./api -run TestDashboard -v`
Expected: FAIL

**Step 3: 实现 dashboard.go**

```go
// go-page-server/api/dashboard.go
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// DashboardHandler 仪表盘 handler
type DashboardHandler struct {
	db      *sqlx.DB
	monitor *core.Monitor
}

// NewDashboardHandler 创建 DashboardHandler
func NewDashboardHandler(db *sqlx.DB, monitor *core.Monitor) *DashboardHandler {
	return &DashboardHandler{db: db, monitor: monitor}
}

// Stats 获取仪表盘统计数据
// GET /api/dashboard/stats
func (h *DashboardHandler) Stats(c *gin.Context) {
	stats := make(map[string]interface{})

	// 站点数量
	var siteCount int
	if err := h.db.Get(&siteCount, "SELECT COUNT(*) FROM sites WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count sites")
	}
	stats["site_count"] = siteCount

	// 关键词数量
	var keywordCount int
	if err := h.db.Get(&keywordCount, "SELECT COUNT(*) FROM keywords WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count keywords")
	}
	stats["keyword_count"] = keywordCount

	// 图片数量
	var imageCount int
	if err := h.db.Get(&imageCount, "SELECT COUNT(*) FROM images WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count images")
	}
	stats["image_count"] = imageCount

	// 文章数量
	var articleCount int
	if err := h.db.Get(&articleCount, "SELECT COUNT(*) FROM original_articles WHERE status = 1"); err != nil {
		log.Warn().Err(err).Msg("Failed to count articles")
	}
	stats["article_count"] = articleCount

	// 模板数量
	var templateCount int
	if err := h.db.Get(&templateCount, "SELECT COUNT(*) FROM templates"); err != nil {
		log.Warn().Err(err).Msg("Failed to count templates")
	}
	stats["template_count"] = templateCount

	core.Success(c, stats)
}

// SpiderVisits 获取蜘蛛访问统计
// GET /api/dashboard/spider-visits
func (h *DashboardHandler) SpiderVisits(c *gin.Context) {
	// 从监控中获取蜘蛛统计
	if h.monitor != nil {
		snapshot := h.monitor.GetCurrentSnapshot()
		if snapshot != nil {
			core.Success(c, gin.H{
				"total_visits": snapshot.SpiderVisits,
				"today_visits": snapshot.SpiderVisits, // 简化，实际需要按天统计
			})
			return
		}
	}

	// 回退到数据库查询
	var todayVisits int
	h.db.Get(&todayVisits, `
		SELECT COUNT(*) FROM spider_logs
		WHERE DATE(visit_time) = CURDATE()
	`)

	core.Success(c, gin.H{
		"total_visits": todayVisits,
		"today_visits": todayVisits,
	})
}

// CacheStats 获取缓存统计
// GET /api/dashboard/cache-stats
func (h *DashboardHandler) CacheStats(c *gin.Context) {
	if h.monitor != nil {
		snapshot := h.monitor.GetCurrentSnapshot()
		if snapshot != nil {
			core.Success(c, gin.H{
				"cache_hits":   snapshot.CacheHits,
				"cache_misses": snapshot.CacheMisses,
				"hit_rate":     snapshot.CacheHitRate,
			})
			return
		}
	}

	core.Success(c, gin.H{
		"cache_hits":   0,
		"cache_misses": 0,
		"hit_rate":     0.0,
	})
}
```

**Step 4: 运行测试确认通过**

Run: `cd go-page-server && go test ./api -run TestDashboard -v`
Expected: PASS

**Step 5: 注册路由**

在 `router.go` 中添加：

```go
// 仪表盘路由（需要认证）
dashboardHandler := NewDashboardHandler(deps.DB, deps.Monitor)
dashboardGroup := r.Group("/api/dashboard")
dashboardGroup.Use(AuthMiddleware(cfg.Auth.SecretKey))
{
	dashboardGroup.GET("/stats", dashboardHandler.Stats)
	dashboardGroup.GET("/spider-visits", dashboardHandler.SpiderVisits)
	dashboardGroup.GET("/cache-stats", dashboardHandler.CacheStats)
}
```

**Step 6: Commit**

```bash
git add go-page-server/api/dashboard.go go-page-server/api/dashboard_test.go go-page-server/api/router.go
git commit -m "feat(api): 添加仪表盘 API - stats/spider-visits/cache-stats"
```

---

## Task 9: 添加日志查询 API

**Files:**
- Create: `go-page-server/api/logs.go`

**Step 1: 实现 logs.go**

```go
// go-page-server/api/logs.go
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// LogsHandler 日志查询 handler
type LogsHandler struct {
	db *sqlx.DB
}

// NewLogsHandler 创建 LogsHandler
func NewLogsHandler(db *sqlx.DB) *LogsHandler {
	return &LogsHandler{db: db}
}

// SystemLog 系统日志结构
type SystemLog struct {
	ID        int    `json:"id" db:"id"`
	Level     string `json:"level" db:"level"`
	Module    string `json:"module" db:"module"`
	Message   string `json:"message" db:"message"`
	CreatedAt string `json:"created_at" db:"created_at"`
}

// History 查询历史日志
// GET /api/logs/history
func (h *LogsHandler) History(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	level := c.Query("level")
	module := c.Query("module")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// 构建查询
	query := "SELECT id, level, module, message, created_at FROM system_logs WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM system_logs WHERE 1=1"
	args := []interface{}{}

	if level != "" {
		query += " AND level = ?"
		countQuery += " AND level = ?"
		args = append(args, level)
	}
	if module != "" {
		query += " AND module = ?"
		countQuery += " AND module = ?"
		args = append(args, module)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	// 获取总数
	var total int64
	if err := h.db.Get(&total, countQuery, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to count logs")
	}

	// 获取列表
	args = append(args, pageSize, offset)
	var logs []SystemLog
	if err := h.db.Select(&logs, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to query logs")
		logs = []SystemLog{}
	}

	core.SuccessPaged(c, logs, total, page, pageSize)
}

// Stats 获取日志统计
// GET /api/logs/stats
func (h *LogsHandler) Stats(c *gin.Context) {
	stats := make(map[string]int)

	rows, err := h.db.Query(`
		SELECT level, COUNT(*) as count
		FROM system_logs
		GROUP BY level
	`)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get log stats")
		core.Success(c, stats)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var level string
		var count int
		rows.Scan(&level, &count)
		stats[level] = count
	}

	core.Success(c, stats)
}

// Clear 清理旧日志
// DELETE /api/logs/clear
func (h *LogsHandler) Clear(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}

	result, err := h.db.Exec(`
		DELETE FROM system_logs
		WHERE created_at < DATE_SUB(NOW(), INTERVAL ? DAY)
	`, days)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "清理失败")
		return
	}

	affected, _ := result.RowsAffected()
	core.Success(c, gin.H{
		"deleted": affected,
		"message": "清理完成",
	})
}
```

**Step 2: 注册路由**

在 `router.go` 中添加：

```go
// 日志路由（需要认证）
logsHandler := NewLogsHandler(deps.DB)
logsGroup := r.Group("/api/logs")
logsGroup.Use(AuthMiddleware(cfg.Auth.SecretKey))
{
	logsGroup.GET("/history", logsHandler.History)
	logsGroup.GET("/stats", logsHandler.Stats)
	logsGroup.DELETE("/clear", logsHandler.Clear)
}
```

**Step 3: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add go-page-server/api/logs.go go-page-server/api/router.go
git commit -m "feat(api): 添加日志查询 API - history/stats/clear"
```

---

## Task 10: 集成测试

**Files:**
- Modify: `go-page-server/api/handler_test.go` (添加集成测试)

**Step 1: 运行所有测试**

Run: `cd go-page-server && go test ./... -v`
Expected: All tests PASS

**Step 2: 验证完整启动**

Run: `cd go-page-server && go build -o server.exe . && echo "Build successful"`
Expected: Build successful

**Step 3: 最终 Commit**

```bash
git add -A
git commit -m "test: Batch 1 完成 - 认证、仪表盘、日志模块"
```

---

## 验收标准

- [ ] `POST /api/auth/login` - 返回 JWT token
- [ ] `POST /api/auth/logout` - 返回成功
- [ ] `GET /api/auth/profile` - 返回用户信息（需认证）
- [ ] `POST /api/auth/change-password` - 修改密码（需认证）
- [ ] `GET /api/dashboard/stats` - 返回统计数据
- [ ] `GET /api/dashboard/spider-visits` - 返回蜘蛛访问统计
- [ ] `GET /api/dashboard/cache-stats` - 返回缓存统计
- [ ] `GET /api/logs/history` - 返回日志列表
- [ ] `GET /api/logs/stats` - 返回日志统计
- [ ] `DELETE /api/logs/clear` - 清理旧日志

---

## 下一批计划

完成 Batch 1 后，继续 Batch 2：
- 关键词管理 (18个接口)
- 图片管理 (17个接口)

见 `docs/plans/2026-01-30-batch2-keywords-images.md`
