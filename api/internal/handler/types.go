package api

import "time"

// SpiderLogItem 蜘蛛访问日志条目 (spider_logs 表)
type SpiderLogItem struct {
	ID         int64     `db:"id" json:"id"`
	SpiderType string    `db:"spider_type" json:"spider_type"`
	IP         string    `db:"ip" json:"ip"`
	UA         string    `db:"ua" json:"ua"`
	Domain     string    `db:"domain" json:"domain"`
	Path       string    `db:"path" json:"path"`
	DnsOk      int       `db:"dns_ok" json:"dns_ok"`
	RespTime   int       `db:"resp_time" json:"resp_time"`
	CacheHit   int       `db:"cache_hit" json:"cache_hit"`
	Status     int       `db:"status" json:"status"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// StatsChartPoint 统计图表数据点 (spider_stats_history 聚合查询)
type StatsChartPoint struct {
	Time      time.Time `db:"time" json:"time"`
	Total     int64     `db:"total" json:"total"`
	Completed int64     `db:"completed" json:"completed"`
	Failed    int64     `db:"failed" json:"failed"`
	Retried   int64     `db:"retried" json:"retried"`
	AvgSpeed  float64   `db:"avg_speed" json:"avg_speed"`
}
