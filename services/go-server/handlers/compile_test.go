// Package handlers contains HTTP request handlers
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestValidateTemplateEndpoint tests the POST /api/template/validate endpoint
func TestValidateTemplateEndpoint(t *testing.T) {
	// Create a mock handler (without database connection)
	handler := &CompileHandler{
		templatesDir: "./templates",
	}

	router := gin.New()
	router.POST("/api/template/validate", handler.ValidateTemplate)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedValid  bool
	}{
		{
			name: "Valid simple template",
			requestBody: map[string]interface{}{
				"content": `<div>{{ title }}</div>`,
			},
			expectedStatus: http.StatusOK,
			expectedValid:  true,
		},
		{
			name: "Valid template with function",
			requestBody: map[string]interface{}{
				"content": `<a href="{{ random_url() }}">Link</a>`,
			},
			expectedStatus: http.StatusOK,
			expectedValid:  true,
		},
		{
			name: "Valid template with for loop",
			requestBody: map[string]interface{}{
				"content": `{% for i in range(10) %}<li>{{ random_hotspot() }}</li>{% endfor %}`,
			},
			expectedStatus: http.StatusOK,
			expectedValid:  true,
		},
		{
			name: "Invalid template - unclosed for loop",
			requestBody: map[string]interface{}{
				"content": `{% for i in range(10) %}<li>item</li>`,
			},
			expectedStatus: http.StatusOK,
			expectedValid:  false,
		},
		{
			name: "Invalid template - undefined function",
			requestBody: map[string]interface{}{
				"content": `{{ undefined_function() }}`,
			},
			expectedStatus: http.StatusOK,
			expectedValid:  false,
		},
		{
			name: "Empty content",
			requestBody: map[string]interface{}{
				"content": "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing content field",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/template/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				valid, ok := response["valid"].(bool)
				if !ok {
					t.Error("Response missing 'valid' field")
				} else if valid != tt.expectedValid {
					t.Errorf("Expected valid=%v, got %v. Errors: %v", tt.expectedValid, valid, response["errors"])
				}
			}
		})
	}
}

// TestPreviewTemplateEndpoint tests the POST /api/template/preview endpoint
func TestPreviewTemplateEndpoint(t *testing.T) {
	handler := &CompileHandler{
		templatesDir: "./templates",
	}

	router := gin.New()
	router.POST("/api/template/preview", handler.PreviewTemplate)

	tests := []struct {
		name                string
		requestBody         map[string]interface{}
		expectedStatus      int
		expectedValid       bool
		expectQuickTemplate bool
	}{
		{
			name: "Valid template conversion",
			requestBody: map[string]interface{}{
				"content": `<div class="{{ cls('header') }}">{{ title }}</div>`,
			},
			expectedStatus:      http.StatusOK,
			expectedValid:       true,
			expectQuickTemplate: true,
		},
		{
			name: "Template with for loop",
			requestBody: map[string]interface{}{
				"content": `{% for i in range(5) %}<li>{{ random_url() }}</li>{% endfor %}`,
			},
			expectedStatus:      http.StatusOK,
			expectedValid:       true,
			expectQuickTemplate: true,
		},
		{
			name: "Invalid template",
			requestBody: map[string]interface{}{
				"content": `{% for i in range(5) %}<li>item</li>`,
			},
			expectedStatus: http.StatusBadRequest,
			expectedValid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api/template/preview", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if tt.expectedStatus == http.StatusOK {
				valid, ok := response["valid"].(bool)
				if !ok {
					t.Error("Response missing 'valid' field")
				} else if valid != tt.expectedValid {
					t.Errorf("Expected valid=%v, got %v", tt.expectedValid, valid)
				}

				if tt.expectQuickTemplate {
					qt, ok := response["quicktemplate"].(string)
					if !ok || qt == "" {
						t.Error("Response missing 'quicktemplate' field")
					}
				}
			}
		})
	}
}

// TestCompileStatusEndpoint tests the GET /api/template/compile/status endpoint
func TestCompileStatusEndpoint(t *testing.T) {
	handler := &CompileHandler{
		templatesDir: "./templates",
	}

	router := gin.New()
	router.GET("/api/template/compile/status", handler.CompileStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/template/compile/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"templates_dir_exists", "templates_dir", "qtc_available", "go_available", "ready"}
	for _, field := range requiredFields {
		if _, ok := response[field]; !ok {
			t.Errorf("Response missing field: %s", field)
		}
	}
}

// TestComplexTemplateValidation tests validation of complex templates
func TestComplexTemplateValidation(t *testing.T) {
	handler := &CompileHandler{
		templatesDir: "./templates",
	}

	router := gin.New()
	router.POST("/api/template/validate", handler.ValidateTemplate)

	// Complex template similar to download_site.html
	complexTemplate := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <title>{{ title }}</title>
    <meta http-equiv="mobile-agent" content="format=xhtml; url={{ random_url() }}" />
</head>
<body>
    <div class="{{ cls('topper') }}">
        <div class="{{ cls('box') }}">
            <span class="{{ cls('fl') }}">{{ keyword_with_emoji() }}</span>
            {% for i in range(4) %}
            <a href="{{ random_url() }}">{{ random_hotspot() }}</a>|
            {% endfor %}
        </div>
    </div>

    <div class="{{ cls('header') }}">
        <div class="{{ cls('logo') }}">
            <a href="{{ random_url() }}"><b>{{ site_id }}</b>.com</a>
        </div>
    </div>

    {% for i in range(10) %}
    <div class="{{ cls('item') }}">
        <img src="{{ random_image() }}" alt="{{ random_hotspot() }}" />
        <p>评分: {{ random_number(1, 10) }}/10</p>
        <p>更新: {{ now() }}</p>
    </div>
    {% endfor %}

    <div class="{{ cls('footer') }}">
        {{ analytics_code or '' }}
        {{ baidu_push_js or '' }}
    </div>
</body>
</html>`

	body, _ := json.Marshal(map[string]interface{}{
		"content": complexTemplate,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/template/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if valid, ok := response["valid"].(bool); !ok || !valid {
		t.Errorf("Complex template should be valid. Errors: %v", response["errors"])
	}
}

// TestPreviewComplexTemplate tests preview of complex templates
func TestPreviewComplexTemplate(t *testing.T) {
	handler := &CompileHandler{
		templatesDir: "./templates",
	}

	router := gin.New()
	router.POST("/api/template/preview", handler.PreviewTemplate)

	template := `<div class="{{ cls('infoRight') }}">
    <dl class="{{ cls('kbox') }}">
        <dt>相关游戏</dt>
        <dd>
            {% for i in range(18) %}
            <a href="{{ random_url() }}">
                <img alt="{{ random_hotspot() }}" src="{{ random_image() }}" />
                <i>{{ random_hotspot() }}</i>
            </a>
            {% endfor %}
        </dd>
    </dl>
</div>`

	body, _ := json.Marshal(map[string]interface{}{
		"content": template,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/template/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check quicktemplate output
	qt, ok := response["quicktemplate"].(string)
	if !ok || qt == "" {
		t.Error("Response missing 'quicktemplate' field")
	}

	// Verify conversion patterns
	expectedPatterns := []string{
		`{%s p.Cls("infoRight") %}`,
		`{% for i := 0; i < 18; i++ %}`,
		`{%s p.RandomURL() %}`,
		`{%s p.RandomImage() %}`,
	}

	for _, pattern := range expectedPatterns {
		if !containsString(qt, pattern) {
			t.Errorf("Missing expected pattern in quicktemplate output: %s", pattern)
		}
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
