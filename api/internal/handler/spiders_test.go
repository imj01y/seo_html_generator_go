package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSpidersList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpidersHandler{}
	r.GET("/api/spider-projects", handler.List)

	req := httptest.NewRequest("GET", "/api/spider-projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestSpidersGet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpidersHandler{}
	r.GET("/api/spider-projects/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/spider-projects/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 无数据库时返回 404
	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", w.Code)
	}
}

func TestSpidersCodeTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpidersHandler{}
	r.GET("/api/spider-projects/templates", handler.GetCodeTemplates)

	req := httptest.NewRequest("GET", "/api/spider-projects/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}

	data, ok := resp["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Error("Expected non-empty templates array")
	}
}

func TestGeneratorsList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &GeneratorsHandler{}
	r.GET("/api/generators", handler.List)

	req := httptest.NewRequest("GET", "/api/generators", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestGeneratorsGet_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &GeneratorsHandler{}
	r.GET("/api/generators/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/generators/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 没有数据库时应该返回 404
	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", w.Code)
	}
}

func TestGeneratorsTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &GeneratorsHandler{}
	r.GET("/api/generators/templates/list", handler.GetTemplates)

	req := httptest.NewRequest("GET", "/api/generators/templates/list", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}

	data, ok := resp["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Error("Expected non-empty templates array")
	}
}

func TestSpiderStatsOverview_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpiderStatsHandler{}
	r.GET("/api/spider-stats/overview", handler.GetOverview)

	req := httptest.NewRequest("GET", "/api/spider-stats/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestSpiderStatsScheduled_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpiderStatsHandler{}
	r.GET("/api/spider-stats/scheduled", handler.GetScheduled)

	req := httptest.NewRequest("GET", "/api/spider-stats/scheduled", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}
