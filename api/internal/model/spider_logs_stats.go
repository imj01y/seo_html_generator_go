package models

import "time"

// SpiderLogsStatsPoint 蜘蛛日志统计数据点
type SpiderLogsStatsPoint struct {
	Time        time.Time `db:"time" json:"time"`
	Total       int       `db:"total" json:"total"`
	Status2xx   int       `db:"status_2xx" json:"status_2xx"`
	Status3xx   int       `db:"status_3xx" json:"status_3xx"`
	Status4xx   int       `db:"status_4xx" json:"status_4xx"`
	Status5xx   int       `db:"status_5xx" json:"status_5xx"`
	AvgRespTime int       `db:"avg_resp_time" json:"avg_resp_time"`
}

// SpiderLogsTrendResponse 趋势接口响应
type SpiderLogsTrendResponse struct {
	Period string                 `json:"period"`
	Items  []SpiderLogsStatsPoint `json:"items"`
}
