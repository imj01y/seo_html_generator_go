package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	core "seo-generator/api/internal/service"
)

// ErrorHandlerMiddleware 统一错误处理中间件
// 处理 Gin 上下文中的错误，将 AppError 转换为标准化的 JSON 响应
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // 先执行后续处理器

		// 检查是否有错误
		if len(c.Errors) == 0 {
			return
		}

		// 处理最后一个错误
		err := c.Errors.Last().Err

		// 区分 AppError 和普通错误
		if appErr := core.GetAppError(err); appErr != nil {
			handleAppError(c, appErr)
		} else {
			handleGenericError(c, err)
		}
	}
}

// handleAppError 处理 AppError 类型的错误
func handleAppError(c *gin.Context, appErr *core.AppError) {
	statusCode := appErr.HTTPStatus()

	// 记录日志
	log.Warn().
		Int("code", int(appErr.Code)).
		Str("message", appErr.Message).
		Int("status", statusCode).
		Err(appErr.Err).
		Msg(appErr.Message)

	response := gin.H{
		"code":    appErr.Code,
		"message": appErr.Message,
	}

	if appErr.Detail != "" {
		response["detail"] = appErr.Detail
	}

	c.JSON(statusCode, response)
}

// handleGenericError 处理普通错误
func handleGenericError(c *gin.Context, err error) {
	log.Error().Err(err).Msg("Internal server error")

	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    core.ErrInternalServer,
		"message": core.GetErrorMessage(core.ErrInternalServer),
	})
}
