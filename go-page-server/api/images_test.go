package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestImagesListGroups_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ImagesHandler{}
	r.GET("/api/images/groups", handler.ListGroups)

	req := httptest.NewRequest("GET", "/api/images/groups", nil)
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

func TestImagesListURLs_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ImagesHandler{}
	r.GET("/api/images/urls/list", handler.ListURLs)

	req := httptest.NewRequest("GET", "/api/images/urls/list?group_id=1&page=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestImagesAddURL_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ImagesHandler{}
	r.POST("/api/images/urls/add", handler.AddURL)

	body := `{"url": "http://example.com/test.jpg", "group_id": 1}`
	req := httptest.NewRequest("POST", "/api/images/urls/add", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500, got %d", w.Code)
	}
}

func TestImagesBatchDelete_EmptyIds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ImagesHandler{}
	r.DELETE("/api/images/batch", handler.BatchDelete)

	body := `{"ids": []}`
	req := httptest.NewRequest("DELETE", "/api/images/batch", strings.NewReader(body))
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

func TestImagesStats_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ImagesHandler{}
	r.GET("/api/images/urls/stats", handler.Stats)

	req := httptest.NewRequest("GET", "/api/images/urls/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestImagesRandom_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ImagesHandler{}
	r.GET("/api/images/urls/random", handler.Random)

	req := httptest.NewRequest("GET", "/api/images/urls/random?count=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}
