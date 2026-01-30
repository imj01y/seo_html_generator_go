// Package handlers contains HTTP request handlers
package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/service"
)

// LogHandler handles spider log requests
type LogHandler struct {
	db             *sqlx.DB
	spiderDetector *core.SpiderDetector
}

// NewLogHandler creates a new log handler
func NewLogHandler(db *sqlx.DB) *LogHandler {
	return &LogHandler{
		db:             db,
		spiderDetector: core.GetSpiderDetector(),
	}
}

// LogSpiderVisit 记录蜘蛛访问日志（供 Nginx Lua 调用）
func (h *LogHandler) LogSpiderVisit(c *gin.Context) {
	ua := c.Query("ua")
	domain := c.Query("domain")
	path := c.Query("path")
	ip := c.Query("ip")
	cacheHitStr := c.DefaultQuery("cache_hit", "1")
	respTimeStr := c.DefaultQuery("resp_time", "0")

	if ua == "" || domain == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
		return
	}

	// 蜘蛛检测
	detection := h.spiderDetector.Detect(ua)
	if !detection.IsSpider {
		c.JSON(http.StatusOK, gin.H{"status": "skipped", "reason": "not spider"})
		return
	}

	cacheHit, _ := strconv.Atoi(cacheHitStr)
	respTime, _ := strconv.Atoi(respTimeStr)

	// 截断过长的值
	if len(ua) > 500 {
		ua = ua[:500]
	}
	if len(path) > 500 {
		path = path[:500]
	}

	dnsOk := 0
	if detection.DNSVerified {
		dnsOk = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `INSERT INTO spider_logs (spider_type, ip, ua, domain, path, dns_ok, resp_time, cache_hit, status)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := h.db.ExecContext(ctx, query, detection.SpiderType, ip, ua, domain, path, dnsOk, respTime, cacheHit, 200)
	if err != nil {
		log.Error().Err(err).Msg("Failed to log spider visit")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	log.Debug().
		Str("spider_type", detection.SpiderType).
		Str("domain", domain).
		Str("path", path).
		Int("cache_hit", cacheHit).
		Msg("Spider log recorded via API")

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
