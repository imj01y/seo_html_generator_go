package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDashboardStats_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware("test-secret"))

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/stats", handler.Stats)

	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestDashboardSpiderVisits_NoMonitor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/spider-visits", handler.SpiderVisits)

	req := httptest.NewRequest("GET", "/api/dashboard/spider-visits", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["code"].(float64) != 0 {
		t.Fatalf("Expected code 0, got %v", resp["code"])
	}
}

func TestDashboardCacheStats_NoMonitor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/cache-stats", handler.CacheStats)

	req := httptest.NewRequest("GET", "/api/dashboard/cache-stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["code"].(float64) != 0 {
		t.Fatalf("Expected code 0, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if _, ok := data["cache_hits"]; !ok {
		t.Error("Response should contain cache_hits")
	}
	if _, ok := data["cache_misses"]; !ok {
		t.Error("Response should contain cache_misses")
	}
	if _, ok := data["hit_rate"]; !ok {
		t.Error("Response should contain hit_rate")
	}
}
