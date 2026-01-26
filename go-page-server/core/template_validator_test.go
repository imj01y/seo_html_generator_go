package core

import (
	"strings"
	"testing"
)

func TestTemplateValidatorBasicSyntax(t *testing.T) {
	validator := NewTemplateValidator()

	tests := []struct {
		name        string
		template    string
		shouldPass  bool
		errorStage  string
	}{
		{
			name:       "valid template",
			template:   `{{ title }} {% for i in range(10) %}{{ random_keyword() }}{% endfor %}`,
			shouldPass: true,
		},
		{
			name:       "unmatched variable tags",
			template:   `{{ title } {% for i in range(10) %}{% endfor %}`,
			shouldPass: false,
			errorStage: "Jinja2 语法检测",
		},
		{
			name:       "unmatched for/endfor",
			template:   `{% for i in range(10) %}{{ random_keyword() }}`,
			shouldPass: false,
			errorStage: "Jinja2 语法检测",
		},
		{
			name:       "unmatched if/endif",
			template:   `{% if true %}yes`,
			shouldPass: false,
			errorStage: "Jinja2 语法检测",
		},
		{
			name:       "unsupported macro",
			template:   `{% macro test() %}{% endmacro %}`,
			shouldPass: false,
			errorStage: "Jinja2 语法检测",
		},
		{
			name:       "unsupported filter",
			template:   `{{ title | upper }}`,
			shouldPass: false,
			errorStage: "Jinja2 语法检测",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.template)
			if tt.shouldPass && !result.Valid {
				t.Errorf("Expected valid but got errors: %v", result.Errors)
			}
			if !tt.shouldPass && result.Valid {
				t.Error("Expected errors but template was valid")
			}
			if !tt.shouldPass && len(result.Errors) > 0 && result.Errors[0].Stage != tt.errorStage {
				t.Errorf("Expected error stage '%s' but got '%s'", tt.errorStage, result.Errors[0].Stage)
			}
		})
	}
}

func TestTemplateValidatorFunctionWhitelist(t *testing.T) {
	validator := NewTemplateValidator()

	tests := []struct {
		name       string
		template   string
		shouldPass bool
	}{
		{
			name:       "allowed function cls",
			template:   `class="{{ cls('header') }}"`,
			shouldPass: true,
		},
		{
			name:       "allowed function random_url",
			template:   `href="{{ random_url() }}"`,
			shouldPass: true,
		},
		{
			name:       "allowed function random_keyword",
			template:   `{{ random_keyword() }}`,
			shouldPass: true,
		},
		{
			name:       "allowed alias random_hotspot",
			template:   `{{ random_hotspot() }}`,
			shouldPass: true,
		},
		{
			name:       "allowed function random_number",
			template:   `{{ random_number(1, 100) }}`,
			shouldPass: true,
		},
		{
			name:       "disallowed function",
			template:   `{{ unknown_func() }}`,
			shouldPass: false,
		},
		{
			name:       "disallowed function custom",
			template:   `{{ my_custom_func('test') }}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.template)
			if tt.shouldPass && !result.Valid {
				t.Errorf("Expected valid but got errors: %v", result.Errors)
			}
			if !tt.shouldPass && result.Valid {
				t.Error("Expected errors but template was valid")
			}
		})
	}
}

func TestTemplateValidatorVariableWhitelist(t *testing.T) {
	validator := NewTemplateValidator()

	tests := []struct {
		name       string
		template   string
		shouldPass bool
	}{
		{
			name:       "allowed variable title",
			template:   `<title>{{ title }}</title>`,
			shouldPass: true,
		},
		{
			name:       "allowed variable site_id",
			template:   `ID: {{ site_id }}`,
			shouldPass: true,
		},
		{
			name:       "allowed variable with or",
			template:   `{{ analytics_code or '' }}`,
			shouldPass: true,
		},
		{
			name:       "disallowed variable",
			template:   `{{ unknown_var }}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.template)
			if tt.shouldPass && !result.Valid {
				t.Errorf("Expected valid but got errors: %v", result.Errors)
			}
			if !tt.shouldPass && result.Valid {
				t.Error("Expected errors but template was valid")
			}
		})
	}
}

func TestTemplateValidatorComplexTemplate(t *testing.T) {
	validator := NewTemplateValidator()

	// Complex template with multiple features
	template := `<!DOCTYPE html>
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
    <div class="{{ cls('content') }}">
        {% for i in range(50) %}
        <img src="{{ random_image() }}" alt="{{ random_keyword() }}" />
        <p>评分: {{ random_number(1, 10) }}/10</p>
        {% endfor %}
    </div>
    <p>Site ID: {{ site_id }}</p>
    <p>Time: {{ now() }}</p>
    {{ analytics_code or '' }}
    {{ baidu_push_js or '' }}
</body>
</html>`

	result := validator.Validate(template)
	if !result.Valid {
		t.Errorf("Complex template should be valid but got errors: %v", result.Errors)
	}
}

func TestTemplateValidatorErrorLineNumber(t *testing.T) {
	validator := NewTemplateValidator()

	template := `<html>
<head>
    <title>{{ title }}</title>
</head>
<body>
    {{ unknown_func() }}
</body>
</html>`

	result := validator.Validate(template)
	if result.Valid {
		t.Error("Expected error for unknown function")
	}

	if len(result.Errors) > 0 && result.Errors[0].Line == 0 {
		t.Log("Note: Line number detection found no line")
	}

	// The error should mention the function name
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "unknown_func") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Error message should mention 'unknown_func'")
	}
}

func TestTemplateValidatorAddWhitelist(t *testing.T) {
	validator := NewTemplateValidator()

	// Initially should fail
	template := `{{ custom_func() }}`
	result := validator.Validate(template)
	if result.Valid {
		t.Error("Expected error for custom function before adding to whitelist")
	}

	// Add to whitelist
	validator.AddAllowedFunction("custom_func")

	// Now should pass
	result = validator.Validate(template)
	if !result.Valid {
		t.Error("Expected success after adding custom function to whitelist")
	}
}

func TestValidateFunctionsExported(t *testing.T) {
	// Test the exported function
	validTemplate := `{{ random_url() }} {{ cls('test') }}`
	err := ValidateFunctions(validTemplate)
	if err != nil {
		t.Errorf("Expected no error for valid template: %v", err)
	}

	invalidTemplate := `{{ undefined_func() }}`
	err = ValidateFunctions(invalidTemplate)
	if err == nil {
		t.Error("Expected error for invalid template")
	}
}
