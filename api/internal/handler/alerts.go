package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AlertsHandler 告警管理处理器
type AlertsHandler struct{}

// NewAlertsHandler 创建告警处理器
func NewAlertsHandler() *AlertsHandler {
	return &AlertsHandler{}
}

// ContentPoolAlert 内容池告警状态
type ContentPoolAlert struct {
	Level     string    `json:"level"`      // normal, warning, critical, exhausted, unknown
	Message   string    `json:"message"`
	PoolSize  int       `json:"pool_size"`
	UsedSize  int       `json:"used_size"`
	Total     int       `json:"total"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetContentPoolAlert 获取内容池告警状态
// GET /api/alerts/content-pool
func (h *AlertsHandler) GetContentPoolAlert(c *gin.Context) {
	// 目前返回正常状态，实际应从内容池服务获取
	alert := ContentPoolAlert{
		Level:     "normal",
		Message:   "内容池状态正常",
		PoolSize:  1000,
		UsedSize:  0,
		Total:     1000,
		UpdatedAt: time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"alert":   alert,
	})
}

// ResetContentPool 重置内容池
// POST /api/alerts/content-pool/reset
func (h *AlertsHandler) ResetContentPool(c *gin.Context) {
	// 目前返回成功，实际应执行内容池重置逻辑
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "内容池已重置",
		"count":   0,
	})
}
