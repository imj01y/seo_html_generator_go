package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoginHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authHandler := NewAuthHandler("test-secret", 1440, nil)
	r.POST("/api/auth/login", authHandler.Login)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestLogoutHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authHandler := NewAuthHandler("test-secret", 1440, nil)
	r.POST("/api/auth/logout", authHandler.Logout)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
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
