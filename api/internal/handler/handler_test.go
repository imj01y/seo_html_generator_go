package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"seo-generator/api/pkg/config"
	"seo-generator/api/internal/service"
)

// setupTestRouter 创建测试路由器和依赖
func setupTestRouter() (*gin.Engine, *Dependencies) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// 创建测试配置
	testConfig := &config.Config{
		Auth: config.AuthConfig{
			SecretKey:                "test-secret-key-for-testing",
			Algorithm:                "HS256",
			AccessTokenExpireMinutes: 60,
		},
	}

	deps := &Dependencies{
		Config:           testConfig,
		TemplateAnalyzer: core.NewTemplateAnalyzer(),
		// 其他依赖可以为 nil 或在具体测试中设置
	}

	return r, deps
}

// apiResponse 用于解析 API 响应
type apiResponse struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// TestPoolHandler_GetPresets 测试获取预设
func TestPoolHandler_GetPresets(t *testing.T) {
	r, deps := setupTestRouter()

	// 设置路由
	SetupRouter(r, deps)

	// 创建请求
	req, err := http.NewRequest(http.MethodGet, "/api/admin/pool/presets", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证响应码
	if resp.Code != int(core.ErrSuccess) {
		t.Errorf("期望响应码 %d，实际 %d", core.ErrSuccess, resp.Code)
	}

	// 验证响应数据不为空
	if len(resp.Data) == 0 {
		t.Error("响应数据不应为空")
	}

	// 解析预设列表
	var presets []map[string]interface{}
	if err := json.Unmarshal(resp.Data, &presets); err != nil {
		t.Fatalf("解析预设列表失败: %v", err)
	}

	// 验证至少有预设配置
	if len(presets) == 0 {
		t.Error("预设列表不应为空")
	}

	t.Logf("获取到 %d 个预设配置", len(presets))
}

// TestTemplateHandler_GetAnalysis 测试获取模板分析
func TestTemplateHandler_GetAnalysis(t *testing.T) {
	r, deps := setupTestRouter()

	// 添加测试数据
	testTemplate := `
		<html>
		<head><title>{{ random_title() }}</title></head>
		<body>
			<div class="{{ cls('test') }}">
				{{ keyword_with_emoji() }}
			</div>
			{% for i in range(5) %}
			<p>{{ random_content() }}</p>
			{% endfor %}
		</body>
		</html>
	`
	deps.TemplateAnalyzer.AnalyzeTemplate("test_template", 1, testTemplate)

	// 设置路由
	SetupRouter(r, deps)

	// 创建请求
	req, err := http.NewRequest(http.MethodGet, "/api/admin/template/analysis", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证响应码
	if resp.Code != int(core.ErrSuccess) {
		t.Errorf("期望响应码 %d，实际 %d", core.ErrSuccess, resp.Code)
	}

	// 解析响应数据
	var analysisData map[string]interface{}
	if err := json.Unmarshal(resp.Data, &analysisData); err != nil {
		t.Fatalf("解析分析数据失败: %v", err)
	}

	// 验证包含 templates 字段
	if _, ok := analysisData["templates"]; !ok {
		t.Error("响应应包含 templates 字段")
	}

	// 验证包含 max_stats 字段
	if _, ok := analysisData["max_stats"]; !ok {
		t.Error("响应应包含 max_stats 字段")
	}

	// 验证包含 stats 字段
	if _, ok := analysisData["stats"]; !ok {
		t.Error("响应应包含 stats 字段")
	}

	t.Logf("模板分析响应: %+v", analysisData)
}

// TestSystemHandler_Health 测试健康检查
func TestSystemHandler_Health(t *testing.T) {
	r, deps := setupTestRouter()

	// 设置路由
	SetupRouter(r, deps)

	// 创建请求
	req, err := http.NewRequest(http.MethodGet, "/api/admin/system/health", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证响应码
	if resp.Code != int(core.ErrSuccess) {
		t.Errorf("期望响应码 %d，实际 %d", core.ErrSuccess, resp.Code)
	}

	// 解析健康检查数据
	var healthData map[string]interface{}
	if err := json.Unmarshal(resp.Data, &healthData); err != nil {
		t.Fatalf("解析健康检查数据失败: %v", err)
	}

	// 验证包含 status 字段
	status, ok := healthData["status"]
	if !ok {
		t.Error("响应应包含 status 字段")
	}

	// 验证 status 值有效
	validStatuses := map[string]bool{"healthy": true, "degraded": true, "unhealthy": true}
	if statusStr, ok := status.(string); !ok || !validStatuses[statusStr] {
		t.Errorf("无效的 status 值: %v", status)
	}

	// 验证包含 checks 字段
	if _, ok := healthData["checks"]; !ok {
		t.Error("响应应包含 checks 字段")
	}

	// 验证包含 time 字段
	if _, ok := healthData["time"]; !ok {
		t.Error("响应应包含 time 字段")
	}

	// 验证包含 version 字段
	if _, ok := healthData["version"]; !ok {
		t.Error("响应应包含 version 字段")
	}

	t.Logf("健康检查状态: %v", status)
}

// TestSystemHandler_Info 测试系统信息
func TestSystemHandler_Info(t *testing.T) {
	r, deps := setupTestRouter()

	// 设置路由
	SetupRouter(r, deps)

	// 创建请求
	req, err := http.NewRequest(http.MethodGet, "/api/admin/system/info", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 验证响应码
	if resp.Code != int(core.ErrSuccess) {
		t.Errorf("期望响应码 %d，实际 %d", core.ErrSuccess, resp.Code)
	}

	// 解析系统信息数据
	var infoData map[string]interface{}
	if err := json.Unmarshal(resp.Data, &infoData); err != nil {
		t.Fatalf("解析系统信息失败: %v", err)
	}

	// 验证包含 runtime 字段
	if _, ok := infoData["runtime"]; !ok {
		t.Error("响应应包含 runtime 字段")
	}

	// 验证包含 memory 字段
	if _, ok := infoData["memory"]; !ok {
		t.Error("响应应包含 memory 字段")
	}

	// 验证包含 uptime 字段
	if _, ok := infoData["uptime"]; !ok {
		t.Error("响应应包含 uptime 字段")
	}

	t.Logf("系统信息: runtime=%v", infoData["runtime"])
}

// TestPoolHandler_GetPresetByName 测试按名称获取预设
func TestPoolHandler_GetPresetByName(t *testing.T) {
	r, deps := setupTestRouter()

	// 设置路由
	SetupRouter(r, deps)

	tests := []struct {
		name       string
		presetName string
		wantCode   int
	}{
		{
			name:       "获取低并发预设",
			presetName: "low",
			wantCode:   http.StatusOK,
		},
		{
			name:       "获取中并发预设",
			presetName: "medium",
			wantCode:   http.StatusOK,
		},
		{
			name:       "获取高并发预设",
			presetName: "high",
			wantCode:   http.StatusOK,
		},
		{
			name:       "获取不存在的预设",
			presetName: "nonexistent",
			wantCode:   http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/api/admin/pool/preset/"+tt.presetName, nil)
			if err != nil {
				t.Fatalf("创建请求失败: %v", err)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("期望状态码 %d，实际 %d", tt.wantCode, w.Code)
			}
		})
	}
}

// TestTemplateHandler_GetAnalysisByID 测试按 ID 获取模板分析
func TestTemplateHandler_GetAnalysisByID(t *testing.T) {
	r, deps := setupTestRouter()

	// 添加测试数据
	deps.TemplateAnalyzer.AnalyzeTemplate("test_tpl", 123, "<div>{{ cls('test') }}</div>")

	// 设置路由
	SetupRouter(r, deps)

	tests := []struct {
		name     string
		url      string
		wantCode int
	}{
		{
			name:     "获取存在的模板分析",
			url:      "/api/admin/template/analysis/123?name=test_tpl",
			wantCode: http.StatusOK,
		},
		{
			name:     "缺少 name 参数",
			url:      "/api/admin/template/analysis/123",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "无效的站点组 ID",
			url:      "/api/admin/template/analysis/invalid?name=test_tpl",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "模板不存在",
			url:      "/api/admin/template/analysis/999?name=nonexistent",
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.url, nil)
			if err != nil {
				t.Fatalf("创建请求失败: %v", err)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("期望状态码 %d，实际 %d，响应: %s", tt.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

// TestPoolHandler_StatsWithNilDeps 测试依赖为 nil 时的错误处理
func TestPoolHandler_StatsWithNilDeps(t *testing.T) {
	r, deps := setupTestRouter()
	// TemplateFuncs 为 nil

	// 设置路由
	SetupRouter(r, deps)

	// 创建请求
	req, err := http.NewRequest(http.MethodGet, "/api/admin/pool/stats", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 应该返回 500 错误
	if w.Code != http.StatusInternalServerError {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusInternalServerError, w.Code)
	}
}

// TestTemplateHandler_PoolConfig 测试获取池配置
func TestTemplateHandler_PoolConfig(t *testing.T) {
	r, deps := setupTestRouter()

	// 添加测试数据以生成池配置
	deps.TemplateAnalyzer.AnalyzeTemplate("tpl1", 1, "<div>{{ cls('x') }}{{ cls('y') }}</div>")

	// 设置路由
	SetupRouter(r, deps)

	// 创建请求
	req, err := http.NewRequest(http.MethodGet, "/api/admin/template/pool-config", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 执行请求
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证状态码
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，实际 %d", http.StatusOK, w.Code)
	}

	// 解析响应
	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	// 解析配置数据
	var configData map[string]interface{}
	if err := json.Unmarshal(resp.Data, &configData); err != nil {
		t.Fatalf("解析配置数据失败: %v", err)
	}

	// 验证包含必要字段
	if _, ok := configData["config"]; !ok {
		t.Error("响应应包含 config 字段")
	}
	if _, ok := configData["memory_estimate"]; !ok {
		t.Error("响应应包含 memory_estimate 字段")
	}
	if _, ok := configData["memory_human"]; !ok {
		t.Error("响应应包含 memory_human 字段")
	}

	t.Logf("池配置: %+v", configData)
}
