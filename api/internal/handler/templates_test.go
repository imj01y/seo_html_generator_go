package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTemplatesList_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware("test-secret"))

	handler := &TemplatesHandler{}
	r.GET("/api/templates", handler.List)

	req := httptest.NewRequest("GET", "/api/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestTemplatesList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.GET("/api/templates", handler.List)

	req := httptest.NewRequest("GET", "/api/templates", nil)
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
	if _, ok := data["list"]; !ok {
		t.Error("Response should contain list field")
	}
	if _, ok := data["total"]; !ok {
		t.Error("Response should contain total field")
	}
}

func TestTemplatesOptions_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.GET("/api/templates/options", handler.Options)

	req := httptest.NewRequest("GET", "/api/templates/options", nil)
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
	if _, ok := data["options"]; !ok {
		t.Error("Response should contain options field")
	}
}

func TestTemplatesGet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.GET("/api/templates/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/templates/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestTemplatesGet_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.GET("/api/templates/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/templates/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 没有数据库时应该返回 404
	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", w.Code)
	}
}

func TestTemplatesGetSites_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.GET("/api/templates/:id/sites", handler.GetSites)

	req := httptest.NewRequest("GET", "/api/templates/invalid/sites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestTemplatesCreate_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.POST("/api/templates", handler.Create)

	req := httptest.NewRequest("POST", "/api/templates", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 没有请求体应该返回 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestTemplatesUpdate_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.PUT("/api/templates/:id", handler.Update)

	req := httptest.NewRequest("PUT", "/api/templates/invalid", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestTemplatesDelete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.DELETE("/api/templates/:id", handler.Delete)

	req := httptest.NewRequest("DELETE", "/api/templates/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}
