package core

import (
	"encoding/json"
	"testing"
)

// 不需要数据库的单元测试
func TestScheduledTask_JSON(t *testing.T) {
	task := ScheduledTask{
		ID:       1,
		Name:     "测试任务",
		TaskType: TaskTypeRefreshData,
		CronExpr: "0 */10 * * * *",
		Enabled:  true,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ScheduledTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Name != task.Name {
		t.Errorf("Expected name %s, got %s", task.Name, decoded.Name)
	}

	if decoded.TaskType != task.TaskType {
		t.Errorf("Expected task type %s, got %s", task.TaskType, decoded.TaskType)
	}

	if decoded.CronExpr != task.CronExpr {
		t.Errorf("Expected cron expr %s, got %s", task.CronExpr, decoded.CronExpr)
	}

	if decoded.Enabled != task.Enabled {
		t.Errorf("Expected enabled %v, got %v", task.Enabled, decoded.Enabled)
	}
}

func TestRefreshDataParams(t *testing.T) {
	params := RefreshDataParams{
		PoolName: "keywords",
		SiteID:   1,
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var decoded RefreshDataParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.PoolName != params.PoolName {
		t.Errorf("Expected pool name %s, got %s", params.PoolName, decoded.PoolName)
	}

	if decoded.SiteID != params.SiteID {
		t.Errorf("Expected site ID %d, got %d", params.SiteID, decoded.SiteID)
	}
}

func TestRefreshDataParams_Parse(t *testing.T) {
	// 测试空参数
	params, err := ParseRefreshDataParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	if params.PoolName != "all" {
		t.Errorf("Expected default pool name 'all', got %s", params.PoolName)
	}

	// 测试有效 JSON
	jsonData := []byte(`{"pool_name": "images", "site_id": 2}`)
	params, err = ParseRefreshDataParams(jsonData)
	if err != nil {
		t.Fatal(err)
	}
	if params.PoolName != "images" {
		t.Errorf("Expected pool name 'images', got %s", params.PoolName)
	}
	if params.SiteID != 2 {
		t.Errorf("Expected site ID 2, got %d", params.SiteID)
	}
}

func TestRefreshTemplateParams(t *testing.T) {
	params := RefreshTemplateParams{
		TemplateName: "default",
		SiteGroupID:  1,
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var decoded RefreshTemplateParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.TemplateName != params.TemplateName {
		t.Errorf("Expected template name %s, got %s", params.TemplateName, decoded.TemplateName)
	}

	if decoded.SiteGroupID != params.SiteGroupID {
		t.Errorf("Expected site group ID %d, got %d", params.SiteGroupID, decoded.SiteGroupID)
	}
}

func TestClearCacheParams(t *testing.T) {
	params := ClearCacheParams{
		CacheType: "html",
		MaxAge:    3600,
		Domain:    "example.com",
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ClearCacheParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.CacheType != params.CacheType {
		t.Errorf("Expected cache type %s, got %s", params.CacheType, decoded.CacheType)
	}

	if decoded.MaxAge != params.MaxAge {
		t.Errorf("Expected max age %d, got %d", params.MaxAge, decoded.MaxAge)
	}

	if decoded.Domain != params.Domain {
		t.Errorf("Expected domain %s, got %s", params.Domain, decoded.Domain)
	}
}

func TestPushURLsParams(t *testing.T) {
	params := PushURLsParams{
		SiteID:       1,
		URLCount:     100,
		SearchEngine: "baidu",
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PushURLsParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.SiteID != params.SiteID {
		t.Errorf("Expected site ID %d, got %d", params.SiteID, decoded.SiteID)
	}

	if decoded.URLCount != params.URLCount {
		t.Errorf("Expected URL count %d, got %d", params.URLCount, decoded.URLCount)
	}

	if decoded.SearchEngine != params.SearchEngine {
		t.Errorf("Expected search engine %s, got %s", params.SearchEngine, decoded.SearchEngine)
	}
}

func TestTaskTypes(t *testing.T) {
	types := []TaskType{
		TaskTypeRefreshData,
		TaskTypeRefreshTemplate,
		TaskTypeClearCache,
		TaskTypePushURLs,
	}

	expectedValues := []string{
		"refresh_data",
		"refresh_template",
		"clear_cache",
		"push_urls",
	}

	for i, tt := range types {
		if string(tt) == "" {
			t.Errorf("Task type should not be empty")
		}
		if string(tt) != expectedValues[i] {
			t.Errorf("Expected task type %s, got %s", expectedValues[i], string(tt))
		}
	}
}

func TestTaskStatus(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusSuccess,
		TaskStatusFailed,
	}

	expectedValues := []string{
		"pending",
		"running",
		"success",
		"failed",
	}

	for i, s := range statuses {
		if string(s) == "" {
			t.Errorf("Task status should not be empty")
		}
		if string(s) != expectedValues[i] {
			t.Errorf("Expected task status %s, got %s", expectedValues[i], string(s))
		}
	}
}

func TestTaskResult(t *testing.T) {
	result := TaskResult{
		Success:  true,
		Message:  "Task completed successfully",
		Duration: 1234,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var decoded TaskResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Success != result.Success {
		t.Errorf("Expected success %v, got %v", result.Success, decoded.Success)
	}

	if decoded.Message != result.Message {
		t.Errorf("Expected message %s, got %s", result.Message, decoded.Message)
	}

	if decoded.Duration != result.Duration {
		t.Errorf("Expected duration %d, got %d", result.Duration, decoded.Duration)
	}
}
