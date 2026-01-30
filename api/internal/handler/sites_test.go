package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSitesList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &SitesHandler{}
	r.GET("/api/sites", handler.List)

	req := httptest.NewRequest("GET", "/api/sites?page=1&page_size=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestSitesCreate_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &SitesHandler{}
	r.POST("/api/sites", handler.Create)

	body := `{"domain": "example.com", "name": "Test Site"}`
	req := httptest.NewRequest("POST", "/api/sites", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500, got %d", w.Code)
	}
}

func TestSitesGet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &SitesHandler{}
	r.GET("/api/sites/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/sites/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestSitesBatchDelete_EmptyIds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &SitesHandler{}
	r.DELETE("/api/sites/batch/delete", handler.BatchDelete)

	body := `{"ids": []}`
	req := httptest.NewRequest("DELETE", "/api/sites/batch/delete", strings.NewReader(body))
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

func TestSiteGroupsList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &SitesHandler{}
	r.GET("/api/site-groups", handler.ListGroups)

	req := httptest.NewRequest("GET", "/api/site-groups", nil)
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

func TestGetAllGroupOptions_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &SitesHandler{}
	r.GET("/api/groups/options", handler.GetAllGroupOptions)

	req := httptest.NewRequest("GET", "/api/groups/options", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}
