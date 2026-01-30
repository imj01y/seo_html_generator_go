package core

import (
	"strings"
	"testing"
)

func TestJinja2ToQuickTemplateConvertFunctions(t *testing.T) {
	converter := NewJinja2ToQuickTemplate("test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "cls function",
			input:    `class="{{ cls('header') }}"`,
			expected: `class="{%s p.Cls("header") %}"`,
		},
		{
			name:     "random_url function",
			input:    `href="{{ random_url() }}"`,
			expected: `href="{%s p.RandomURL() %}"`,
		},
		{
			name:     "random_keyword function",
			input:    `{{ random_keyword() }}`,
			expected: `{%s= string(p.RandomKeyword()) %}`,
		},
		{
			name:     "random_hotspot alias",
			input:    `{{ random_hotspot() }}`,
			expected: `{%s= string(p.RandomKeyword()) %}`,
		},
		{
			name:     "keyword_with_emoji alias",
			input:    `{{ keyword_with_emoji() }}`,
			expected: `{%s= string(p.RandomKeyword()) %}`,
		},
		{
			name:     "random_image function",
			input:    `src="{{ random_image() }}"`,
			expected: `src="{%s p.RandomImage() %}"`,
		},
		{
			name:     "random_number function",
			input:    `{{ random_number(1, 100) }}`,
			expected: `{%d p.RandomNumber(1, 100) %}`,
		},
		{
			name:     "now function",
			input:    `{{ now() }}`,
			expected: `{%s p.Now() %}`,
		},
		{
			name:     "content function",
			input:    `{{ content() }}`,
			expected: `{%s= p.Content %}`,
		},
		{
			name:     "content_with_pinyin alias",
			input:    `{{ content_with_pinyin() }}`,
			expected: `{%s= p.Content %}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.convertFunctions(tt.input)
			if result != tt.expected {
				t.Errorf("Expected: %s\nGot: %s", tt.expected, result)
			}
		})
	}
}

func TestJinja2ToQuickTemplateConvertControlStructures(t *testing.T) {
	converter := NewJinja2ToQuickTemplate("test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "for loop",
			input:    `{% for i in range(10) %}`,
			expected: `{% for i := 0; i < 10; i++ %}`,
		},
		{
			name:     "endfor",
			input:    `{% endfor %}`,
			expected: `{% endfor %}`,
		},
		{
			name:     "if statement",
			input:    `{% if condition %}`,
			expected: `{% if condition %}`,
		},
		{
			name:     "elif to elseif",
			input:    `{% elif other_condition %}`,
			expected: `{% elseif other_condition %}`,
		},
		{
			name:     "else",
			input:    `{% else %}`,
			expected: `{% else %}`,
		},
		{
			name:     "endif",
			input:    `{% endif %}`,
			expected: `{% endif %}`,
		},
		{
			name:     "for with larger number",
			input:    `{% for i in range(400) %}`,
			expected: `{% for i := 0; i < 400; i++ %}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.convertControlStructures(tt.input)
			if result != tt.expected {
				t.Errorf("Expected: %s\nGot: %s", tt.expected, result)
			}
		})
	}
}

func TestJinja2ToQuickTemplateConvertVariables(t *testing.T) {
	converter := NewJinja2ToQuickTemplate("test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "title variable",
			input:    `<title>{{ title }}</title>`,
			expected: `<title>{%s p.Title %}</title>`,
		},
		{
			name:     "site_id variable",
			input:    `ID: {{ site_id }}`,
			expected: `ID: {%d p.SiteID %}`,
		},
		{
			name:     "analytics_code with or",
			input:    `{{ analytics_code or '' }}`,
			expected: `{%s= p.AnalyticsCode %}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply all conversions in order
			result := converter.convertFunctions(tt.input)
			result = converter.convertControlStructures(result)
			result = converter.convertVariablesWithDefault(result)
			result = converter.convertSimpleVariables(result)
			if result != tt.expected {
				t.Errorf("Expected: %s\nGot: %s", tt.expected, result)
			}
		})
	}
}

func TestJinja2ToQuickTemplateConvertComments(t *testing.T) {
	converter := NewJinja2ToQuickTemplate("test")

	input := `<div>{# This is a comment #}</div>`
	expected := `<div></div>`

	result := converter.commentPattern.ReplaceAllString(input, "")
	if result != expected {
		t.Errorf("Expected: %s\nGot: %s", expected, result)
	}
}

func TestJinja2ToQuickTemplateFullConversion(t *testing.T) {
	converter := NewJinja2ToQuickTemplate("test_template")

	jinja2Template := `<!DOCTYPE html>
<html>
<head>
    <title>{{ title }}</title>
</head>
<body>
    <div class="{{ cls('header') }}">
        {% for i in range(4) %}
        <a href="{{ random_url() }}">{{ random_hotspot() }}</a>
        {% endfor %}
    </div>
    <p>Site ID: {{ site_id }}</p>
    <p>Time: {{ now() }}</p>
    {{ analytics_code or '' }}
</body>
</html>`

	result, err := converter.Convert(jinja2Template)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Verify key conversions
	checks := []struct {
		desc    string
		pattern string
	}{
		{"title conversion", `{%s p.Title %}`},
		{"cls conversion", `{%s p.Cls("header") %}`},
		{"for loop conversion", `{% for i := 0; i < 4; i++ %}`},
		{"random_url conversion", `{%s p.RandomURL() %}`},
		{"random_hotspot conversion", `{%s= string(p.RandomKeyword()) %}`},
		{"site_id conversion", `{%d p.SiteID %}`},
		{"now conversion", `{%s p.Now() %}`},
		{"analytics_code conversion", `{%s= p.AnalyticsCode %}`},
		{"endfor conversion", `{% endfor %}`},
		{"quicktemplate header", `{% func (p *PageParams) Render() %}`},
		{"quicktemplate footer", `{% endfunc %}`},
	}

	for _, check := range checks {
		if !strings.Contains(result, check.pattern) {
			t.Errorf("%s: expected pattern '%s' not found in result", check.desc, check.pattern)
		}
	}
}

func TestJinja2ToQuickTemplateValidation(t *testing.T) {
	converter := NewJinja2ToQuickTemplate("test")

	// Valid template
	validTemplate := `{% for i in range(10) %}{{ random_keyword() }}{% endfor %}`
	validResult := converter.ConvertWithValidation(validTemplate)
	if len(validResult.Errors) > 0 {
		t.Errorf("Valid template should not have errors: %v", validResult.Errors)
	}

	// Template with unsupported features
	unsupportedTemplate := `{% macro test() %}{% endmacro %}`
	unsupportedResult := converter.ConvertWithValidation(unsupportedTemplate)
	if len(unsupportedResult.Errors) == 0 {
		t.Error("Template with macro should have errors")
	}
}

func TestValidateJinja2Syntax(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		shouldError bool
	}{
		{
			name:        "valid template",
			template:    `{{ title }} {% for i in range(10) %}{{ random_keyword() }}{% endfor %}`,
			shouldError: false,
		},
		{
			name:        "unmatched variable tags",
			template:    `{{ title } {% for i in range(10) %}{% endfor %}`,
			shouldError: true,
		},
		{
			name:        "unmatched for/endfor",
			template:    `{% for i in range(10) %}{{ random_keyword() }}`,
			shouldError: true,
		},
		{
			name:        "unmatched if/endif",
			template:    `{% if true %}yes`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJinja2Syntax(tt.template)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"title", "Title"},
		{"site_id", "SiteId"},
		{"analytics_code", "AnalyticsCode"},
		{"baidu_push_js", "BaiduPushJs"},
		{"article_content", "ArticleContent"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("Expected: %s, Got: %s", tt.expected, result)
			}
		})
	}
}

// Test converting the actual download_site.html template snippet
func TestConvertRealTemplateSnippet(t *testing.T) {
	converter := NewJinja2ToQuickTemplate("download_site")

	snippet := `<div class="{{ cls('topper') }}">
    <div class="{{ cls('box') }}">
        <span class="{{ cls('fl') }}">{{ keyword_with_emoji() }}</span>
        {% for i in range(4) %}
        <a href="{{ random_url() }}">{{ random_hotspot() }}</a>|
        {% endfor %}
    </div>
</div>`

	result, err := converter.Convert(snippet)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Verify conversions
	expectedPatterns := []string{
		`{%s p.Cls("topper") %}`,
		`{%s p.Cls("box") %}`,
		`{%s p.Cls("fl") %}`,
		`{%s= string(p.RandomKeyword()) %}`,
		`{% for i := 0; i < 4; i++ %}`,
		`{%s p.RandomURL() %}`,
		`{% endfor %}`,
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(result, pattern) {
			t.Errorf("Expected pattern '%s' not found in result", pattern)
		}
	}

	t.Logf("Converted result:\n%s", result)
}
