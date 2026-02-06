package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	core "seo-generator/api/internal/service"
	"seo-generator/api/pkg/config"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			core.AbortWithMessage(c, core.ErrUnauthorized, "缺少认证信息")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			core.AbortWithMessage(c, core.ErrUnauthorized, "认证格式错误")
			return
		}

		token := parts[1]

		claims, err := core.VerifyToken(token, secret)
		if err != nil {
			if err == core.ErrTokenExpired {
				core.AbortWithMessage(c, core.ErrUnauthorized, "Token 已过期")
			} else {
				core.AbortWithMessage(c, core.ErrUnauthorized, "无效的 Token")
			}
			return
		}

		c.Set("claims", claims)
		c.Set("admin_id", claims["admin_id"])
		c.Set("username", claims["sub"])

		c.Next()
	}
}

// DualAuthMiddleware 双轨认证中间件
// 同时支持 JWT 和 API Token 认证，任一通过即可
func DualAuthMiddleware(secret string, db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 尝试 JWT 认证
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				claims, err := core.VerifyToken(parts[1], secret)
				if err == nil {
					c.Set("claims", claims)
					c.Set("admin_id", claims["admin_id"])
					c.Set("username", claims["sub"])
					c.Set("auth_type", "jwt")
					c.Next()
					return
				}
			}
		}

		// 2. 尝试 API Token 认证
		apiToken := c.GetHeader("X-API-Token")
		if apiToken == "" && authHeader != "" {
			// 允许通过 Authorization: Bearer <api_token> 传递
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				apiToken = parts[1]
			}
		}

		if apiToken == "" {
			core.AbortWithMessage(c, core.ErrUnauthorized, "缺少认证信息")
			return
		}

		// 检查 API Token 是否启用
		var enabled string
		if err := db.Get(&enabled, "SELECT setting_value FROM system_settings WHERE setting_key = 'api_token_enabled'"); err != nil || (enabled != "true" && enabled != "1") {
			core.AbortWithMessage(c, core.ErrUnauthorized, "API Token 认证未启用")
			return
		}

		// 验证 API Token
		var storedToken string
		if err := db.Get(&storedToken, "SELECT setting_value FROM system_settings WHERE setting_key = 'api_token'"); err != nil || storedToken == "" {
			core.AbortWithMessage(c, core.ErrUnauthorized, "API Token 未配置")
			return
		}

		if apiToken != storedToken {
			core.AbortWithMessage(c, core.ErrUnauthorized, "无效的 API Token")
			return
		}

		c.Set("auth_type", "api_token")
		c.Next()
	}
}

// DependencyInjectionMiddleware 依赖注入中间件
// 将数据库、Redis 连接、配置和调度器注入到 Gin context 中，供 Handler 使用
func DependencyInjectionMiddleware(db *sqlx.DB, rdb *redis.Client, cfg *config.Config, scheduler *core.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db != nil {
			c.Set("db", db)
		}
		if rdb != nil {
			c.Set("redis", rdb)
		}
		if cfg != nil {
			c.Set("config", cfg)
		}
		if scheduler != nil {
			c.Set("scheduler", scheduler)
		}
		c.Next()
	}
}
