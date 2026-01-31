package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	core "seo-generator/api/internal/service"
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

	if h.db == nil {
		core.SuccessPaged(c, []SystemLog{}, 0, page, pageSize)
		return
	}

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

	if h.db == nil {
		core.Success(c, stats)
		return
	}

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

	if h.db == nil {
		core.Success(c, gin.H{
			"deleted": 0,
			"message": "数据库未初始化",
		})
		return
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
