package core

import (
	"go-page-server/templates"
)

// QuickTemplateRenderer is a high-performance renderer using quicktemplate
type QuickTemplateRenderer struct {
	funcsManager *TemplateFuncsManager
}

// NewQuickTemplateRenderer creates a new quicktemplate renderer
func NewQuickTemplateRenderer(fm *TemplateFuncsManager) *QuickTemplateRenderer {
	return &QuickTemplateRenderer{
		funcsManager: fm,
	}
}

// Render renders the page using quicktemplate
func (r *QuickTemplateRenderer) Render(data *RenderData, content string) string {
	// Create page params with function providers
	params := templates.NewPageParams(
		data.Title,
		data.SiteID,
		string(data.AnalyticsCode),
		string(data.BaiduPushJS),
		string(data.ArticleContent),
		content,
		r.funcsManager.Cls,
		r.funcsManager.RandomURL,
		r.funcsManager.RandomKeyword,
		r.funcsManager.RandomImage,
		r.funcsManager.RandomNumber,
		NowFunc,
	)

	// Render returns the HTML string directly
	return params.Render()
}

// RenderToBytes renders to a byte slice (more efficient for HTTP responses)
func (r *QuickTemplateRenderer) RenderToBytes(data *RenderData, content string) []byte {
	return []byte(r.Render(data, content))
}
