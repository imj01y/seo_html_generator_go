package api

import "github.com/gin-gonic/gin"

// SpidersHandler 爬虫处理器（兼容层）
// 此文件提供向后兼容性，所有功能已拆分到专门的处理器中：
// - SpiderProjectsHandler: 项目 CRUD
// - SpiderFilesHandler: 文件管理
// - SpiderExecutionHandler: 执行控制
// - SpiderStatsHandler: 统计和日志
type SpidersHandler struct {
	Projects  *SpiderProjectsHandler
	Files     *SpiderFilesHandler
	Execution *SpiderExecutionHandler
	Stats     *SpiderStatsHandler
}

// NewSpidersHandler 创建 SpidersHandler 实例
func NewSpidersHandler() *SpidersHandler {
	return &SpidersHandler{
		Projects:  &SpiderProjectsHandler{},
		Files:     &SpiderFilesHandler{},
		Execution: &SpiderExecutionHandler{},
		Stats:     &SpiderStatsHandler{},
	}
}

// ============================================
// 项目 CRUD API（代理到 SpiderProjectsHandler）
// ============================================

func (h *SpidersHandler) List(c *gin.Context) {
	h.Projects.List(c)
}

func (h *SpidersHandler) Get(c *gin.Context) {
	h.Projects.Get(c)
}

func (h *SpidersHandler) Create(c *gin.Context) {
	h.Projects.Create(c)
}

func (h *SpidersHandler) Update(c *gin.Context) {
	h.Projects.Update(c)
}

func (h *SpidersHandler) Delete(c *gin.Context) {
	h.Projects.Delete(c)
}

func (h *SpidersHandler) Toggle(c *gin.Context) {
	h.Projects.Toggle(c)
}

func (h *SpidersHandler) GetCodeTemplates(c *gin.Context) {
	h.Projects.GetCodeTemplates(c)
}

// ============================================
// 文件管理 API（代理到 SpiderFilesHandler）
// ============================================

func (h *SpidersHandler) ListFiles(c *gin.Context) {
	h.Files.ListFiles(c)
}

func (h *SpidersHandler) GetFileTree(c *gin.Context) {
	h.Files.GetFileTree(c)
}

func (h *SpidersHandler) GetFile(c *gin.Context) {
	h.Files.GetFile(c)
}

func (h *SpidersHandler) CreateItem(c *gin.Context) {
	h.Files.CreateItem(c)
}

func (h *SpidersHandler) UpdateFile(c *gin.Context) {
	h.Files.UpdateFile(c)
}

func (h *SpidersHandler) DeleteFile(c *gin.Context) {
	h.Files.DeleteFile(c)
}

func (h *SpidersHandler) MoveItem(c *gin.Context) {
	h.Files.MoveItem(c)
}

// ============================================
// 执行控制 API（代理到 SpiderExecutionHandler）
// ============================================

func (h *SpidersHandler) Run(c *gin.Context) {
	h.Execution.Run(c)
}

func (h *SpidersHandler) Test(c *gin.Context) {
	h.Execution.Test(c)
}

func (h *SpidersHandler) TestStop(c *gin.Context) {
	h.Execution.TestStop(c)
}

func (h *SpidersHandler) Stop(c *gin.Context) {
	h.Execution.Stop(c)
}

func (h *SpidersHandler) Pause(c *gin.Context) {
	h.Execution.Pause(c)
}

func (h *SpidersHandler) Resume(c *gin.Context) {
	h.Execution.Resume(c)
}

// ============================================
// 统计和队列管理 API（代理到 SpiderStatsHandler）
// ============================================

func (h *SpidersHandler) GetRealtimeStats(c *gin.Context) {
	h.Stats.GetRealtimeStats(c)
}

func (h *SpidersHandler) GetChartStats(c *gin.Context) {
	h.Stats.GetChartStats(c)
}

func (h *SpidersHandler) ClearQueue(c *gin.Context) {
	h.Stats.ClearQueue(c)
}

func (h *SpidersHandler) Reset(c *gin.Context) {
	h.Stats.Reset(c)
}

func (h *SpidersHandler) ListFailed(c *gin.Context) {
	h.Stats.ListFailed(c)
}

func (h *SpidersHandler) GetFailedStats(c *gin.Context) {
	h.Stats.GetFailedStats(c)
}

func (h *SpidersHandler) RetryAllFailed(c *gin.Context) {
	h.Stats.RetryAllFailed(c)
}

func (h *SpidersHandler) RetryOneFailed(c *gin.Context) {
	h.Stats.RetryOneFailed(c)
}

func (h *SpidersHandler) IgnoreFailed(c *gin.Context) {
	h.Stats.IgnoreFailed(c)
}

func (h *SpidersHandler) DeleteFailed(c *gin.Context) {
	h.Stats.DeleteFailed(c)
}
