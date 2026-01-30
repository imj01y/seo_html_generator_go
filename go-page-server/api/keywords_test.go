package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestKeywordsListGroups_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &KeywordsHandler{}
	r.GET("/api/keywords/groups", handler.ListGroups)

	req := httptest.NewRequest("GET", "/api/keywords/groups", nil)
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

func TestKeywordsList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &KeywordsHandler{}
	r.GET("/api/keywords/list", handler.List)

	req := httptest.NewRequest("GET", "/api/keywords/list?group_id=1&page=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestKeywordsAdd_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &KeywordsHandler{}
	r.POST("/api/keywords/add", handler.Add)

	body := `{"keyword": "test", "group_id": 1}`
	req := httptest.NewRequest("POST", "/api/keywords/add", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 没有数据库应返回 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500, got %d", w.Code)
	}
}

func TestKeywordsBatchDelete_EmptyIds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &KeywordsHandler{}
	r.DELETE("/api/keywords/batch", handler.BatchDelete)

	body := `{"ids": []}`
	req := httptest.NewRequest("DELETE", "/api/keywords/batch", strings.NewReader(body))
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

func TestKeywordsStats_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &KeywordsHandler{}
	r.GET("/api/keywords/stats", handler.Stats)

	req := httptest.NewRequest("GET", "/api/keywords/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestKeywordsRandom_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &KeywordsHandler{}
	r.GET("/api/keywords/random", handler.Random)

	req := httptest.NewRequest("GET", "/api/keywords/random?count=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}
