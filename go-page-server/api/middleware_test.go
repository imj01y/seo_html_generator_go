package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-page-server/core"
)

func TestAuthMiddleware_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware("test-secret"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"

	r := gin.New()
	r.Use(AuthMiddleware(secret))
	r.GET("/test", func(c *gin.Context) {
		claims, _ := c.Get("claims")
		c.JSON(200, claims)
	})

	token, _ := core.CreateAccessToken(map[string]interface{}{
		"sub":      "admin",
		"admin_id": 1,
	}, secret, time.Hour)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"

	r := gin.New()
	r.Use(AuthMiddleware(secret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	token, _ := core.CreateAccessToken(map[string]interface{}{
		"sub": "admin",
	}, secret, -time.Hour)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}
