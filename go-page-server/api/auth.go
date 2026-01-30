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
	db            *sqlx.DB
	secret        string
	expireMinutes int
}

// NewAuthHandler 创建 AuthHandler
func NewAuthHandler(secret string, expireMinutes int, db *sqlx.DB) *AuthHandler {
	return &AuthHandler{
		db:            db,
		secret:        secret,
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
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var admin struct {
		ID        int        `db:"id"`
		Username  string     `db:"username"`
		Password  string     `db:"password"`
		LastLogin *time.Time `db:"last_login"`
	}

	err := h.db.Get(&admin, "SELECT id, username, password, last_login FROM admins WHERE username = ?", req.Username)
	if err != nil {
		log.Debug().Str("username", req.Username).Msg("Admin not found")
		core.Success(c, LoginResponse{Success: false, Message: "用户名或密码错误"})
		return
	}

	if !core.VerifyPassword(req.Password, admin.Password) {
		log.Debug().Str("username", req.Username).Msg("Invalid password")
		core.Success(c, LoginResponse{Success: false, Message: "用户名或密码错误"})
		return
	}

	h.db.Exec("UPDATE admins SET last_login = NOW() WHERE id = ?", admin.ID)

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

	core.Success(c, LoginResponse{Success: true, Token: token, Message: "登录成功"})
}

// Logout 退出登录
func (h *AuthHandler) Logout(c *gin.Context) {
	core.Success(c, gin.H{"success": true})
}

// Profile 获取当前用户信息
func (h *AuthHandler) Profile(c *gin.Context) {
	claims, exists := c.Get("claims")
	if !exists {
		core.FailWithCode(c, core.ErrUnauthorized)
		return
	}

	claimsMap := claims.(map[string]interface{})
	username := claimsMap["sub"].(string)

	if h.db == nil {
		core.Success(c, gin.H{"username": username, "role": "admin", "last_login": nil})
		return
	}

	var admin struct {
		ID        int        `db:"id"`
		Username  string     `db:"username"`
		LastLogin *time.Time `db:"last_login"`
	}

	err := h.db.Get(&admin, "SELECT id, username, last_login FROM admins WHERE username = ?", username)
	if err != nil {
		core.Success(c, gin.H{"username": username, "role": "admin", "last_login": nil})
		return
	}

	var lastLogin interface{}
	if admin.LastLogin != nil {
		lastLogin = admin.LastLogin.Format(time.RFC3339)
	}

	core.Success(c, gin.H{"id": admin.ID, "username": admin.Username, "role": "admin", "last_login": lastLogin})
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	claims, _ := c.Get("claims")
	claimsMap := claims.(map[string]interface{})
	username := claimsMap["sub"].(string)
	adminID := int(claimsMap["admin_id"].(float64))

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

	newHash, err := core.HashPassword(req.NewPassword)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "密码加密失败")
		return
	}

	_, err = h.db.Exec("UPDATE admins SET password = ? WHERE id = ?", newHash, adminID)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "密码更新失败")
		return
	}

	core.Success(c, gin.H{"success": true, "message": "密码修改成功"})
}
