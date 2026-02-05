package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"seo-generator/api/pkg/config"
	core "seo-generator/api/internal/service"
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
