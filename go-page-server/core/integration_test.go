package core

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFullConversionPipeline tests the complete Jinja2 → quicktemplate conversion
func TestFullConversionPipeline(t *testing.T) {
	// Sample Jinja2 template (simplified version of download_site.html)
	jinja2Template := `<!DOCTYPE html>
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

	// Step 1: Validate Jinja2 syntax
	t.Run("Step1_Validate", func(t *testing.T) {
		validator := NewTemplateValidator()
		result := validator.Validate(jinja2Template)
		if !result.Valid {
			t.Fatalf("Validation failed: %v", result.Errors)
		}
		t.Log("✓ Jinja2 syntax validation passed")
	})

	// Step 2: Convert to quicktemplate
	var qtplContent string
	t.Run("Step2_Convert", func(t *testing.T) {
		converter := NewJinja2ToQuickTemplate("integration_test")
		var err error
		qtplContent, err = converter.Convert(jinja2Template)
		if err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		// Verify key patterns exist
		patterns := []string{
			`{%s p.Title %}`,
			`{%s p.Cls("topper") %}`,
			`{% for i := 0; i < 4; i++ %}`,
			`{%s p.RandomURL() %}`,
			`{%s= string(p.RandomKeyword()) %}`,
			`{%d p.SiteID %}`,
			`{%s p.RandomImage() %}`,
			`{%d p.RandomNumber(1, 10) %}`,
			`{%s p.Now() %}`,
			`{% endfunc %}`,
		}

		for _, pattern := range patterns {
			if !containsHelper(qtplContent, pattern) {
				t.Errorf("Missing expected pattern: %s", pattern)
			}
		}
		t.Log("✓ Conversion to quicktemplate completed")
	})

	// Step 3: Save and compile (if in proper environment)
	t.Run("Step3_SaveQtpl", func(t *testing.T) {
		// Create temp directory
		tmpDir, err := os.MkdirTemp("", "qtpl_test")
		if err != nil {
			t.Skipf("Could not create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Save .qtpl file
		qtplPath := filepath.Join(tmpDir, "integration_test.qtpl")
		if err := os.WriteFile(qtplPath, []byte(qtplContent), 0644); err != nil {
			t.Fatalf("Could not write .qtpl file: %v", err)
		}
		t.Logf("✓ Saved .qtpl file to %s", qtplPath)

		// Verify file exists and has content
		info, err := os.Stat(qtplPath)
		if err != nil {
			t.Fatalf("Could not stat .qtpl file: %v", err)
		}
		if info.Size() == 0 {
			t.Error("Saved .qtpl file is empty")
		}
		t.Logf("✓ File size: %d bytes", info.Size())
	})
}

// TestConversionWithRealTemplate tests conversion of a larger template snippet
func TestConversionWithRealTemplate(t *testing.T) {
	// A more complex template snippet similar to download_site.html
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

    <div class="{{ cls('tit') }}">热门冒险解谜</div>
    <ul class="{{ cls('topList') }}">
        {% for i in range(400) %}
        <li>
            <i>{{ now() }}</i>
            <b class='on'>{{ random_number(100, 999) }}</b>
            <a href="{{ random_url() }}" target="_blank">
                <img src="{{ random_image() }}" alt="{{ random_hotspot() }}" />
            </a>
            <p><a href="{{ random_url() }}" target="_blank">{{ random_hotspot() }}</a></p>
        </li>
        {% endfor %}
    </ul>
</div>`

	// Validate
	validator := NewTemplateValidator()
	result := validator.Validate(template)
	if !result.Valid {
		t.Fatalf("Template validation failed: %v", result.Errors)
	}

	// Convert
	converter := NewJinja2ToQuickTemplate("real_template")
	qtpl, err := converter.Convert(template)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Check for 400-iteration loop
	if !containsHelper(qtpl, "{% for i := 0; i < 400; i++ %}") {
		t.Error("400-iteration loop not properly converted")
	}

	// Check for 18-iteration loop
	if !containsHelper(qtpl, "{% for i := 0; i < 18; i++ %}") {
		t.Error("18-iteration loop not properly converted")
	}

	t.Log("✓ Real template snippet conversion successful")
}

// TestValidatorWithInvalidTemplates tests various invalid template scenarios
func TestValidatorWithInvalidTemplates(t *testing.T) {
	validator := NewTemplateValidator()

	invalidTemplates := []struct {
		name     string
		template string
		errStage string
	}{
		{
			name:     "Unclosed for loop",
			template: `{% for i in range(10) %}{{ random_url() }}`,
			errStage: "Jinja2 语法检测",
		},
		{
			name:     "Unclosed if",
			template: `{% if true %}hello`,
			errStage: "Jinja2 语法检测",
		},
		{
			name:     "Undefined function",
			template: `{{ undefined_func() }}`,
			errStage: "函数白名单检测",
		},
		{
			name:     "Unsupported filter",
			template: `{{ title | upper }}`,
			errStage: "Jinja2 语法检测",
		},
		{
			name:     "Unsupported macro",
			template: `{% macro hello() %}{% endmacro %}`,
			errStage: "Jinja2 语法检测",
		},
	}

	for _, tt := range invalidTemplates {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.template)
			if result.Valid {
				t.Errorf("Expected template to be invalid")
			}
			if len(result.Errors) > 0 && result.Errors[0].Stage != tt.errStage {
				t.Errorf("Expected error stage '%s', got '%s'", tt.errStage, result.Errors[0].Stage)
			}
		})
	}
}
