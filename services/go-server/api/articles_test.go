package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestArticlesListGroups_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.GET("/api/articles/groups", handler.ListGroups)

	req := httptest.NewRequest("GET", "/api/articles/groups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Fatalf("Expected code 0, got %v", resp["code"])
	}
}

func TestArticlesList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.GET("/api/articles/list", handler.List)

	req := httptest.NewRequest("GET", "/api/articles/list?group_id=1&page=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestArticlesAdd_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.POST("/api/articles/add", handler.Add)

	body := `{"title": "test", "content": "test content", "group_id": 1}`
	req := httptest.NewRequest("POST", "/api/articles/add", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500, got %d", w.Code)
	}
}

func TestArticlesBatchDelete_EmptyIds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.DELETE("/api/articles/batch/delete", handler.BatchDelete)

	body := `{"ids": []}`
	req := httptest.NewRequest("DELETE", "/api/articles/batch/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["success"].(bool) != false {
		t.Fatal("Expected success: false for empty ids")
	}
}

func TestArticlesGet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.GET("/api/articles/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/articles/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}
