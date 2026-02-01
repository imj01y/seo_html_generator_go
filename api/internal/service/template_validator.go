package core

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError represents a validation error with context
type ValidationError struct {
	Stage      string // Which validation stage failed
	Line       int    // Line number (0 if not applicable)
	Column     int    // Column number (0 if not applicable)
	Message    string // Error message
	Suggestion string // Suggested fix
}

func (e *ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("[%s] 第 %d 行: %s", e.Stage, e.Line, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Stage, e.Message)
}

// ValidationResult contains the result of template validation
type ValidationResult struct {
	Valid    bool               // Whether the template is valid
	Errors   []*ValidationError // List of errors
	Warnings []string           // Non-fatal warnings
}

// TemplateValidator validates Jinja2 templates before conversion
type TemplateValidator struct {
	// Allowed functions whitelist
	allowedFunctions map[string]bool

	// Allowed variables whitelist
	allowedVariables map[string]bool

	// Patterns for extracting function and variable names
	funcCallPattern *regexp.Regexp
	varRefPattern   *regexp.Regexp
}

// NewTemplateValidator creates a new validator with default whitelists
func NewTemplateValidator() *TemplateValidator {
	return &TemplateValidator{
		allowedFunctions: map[string]bool{
			// Core functions
			"cls":            true,
			"random_url":     true,
			"random_keyword": true,
			"random_image":   true,
			"random_number":  true,
			"encode":         true,
			"content":        true,
			"now":            true,

			// Alias functions (mapped to core functions)
			"random_hotspot":      true, // → random_keyword
			"keyword_with_emoji":  true, // → random_keyword
			"content_with_pinyin": true, // → content
			"encode_text":         true, // → encode
		},
		allowedVariables: map[string]bool{
			"title":           true,
			"site_id":         true,
			"analytics_code":  true,
			"baidu_push_js":   true,
			"article_content": true,
			"i":               true, // Loop variable
		},
		funcCallPattern: regexp.MustCompile(`\{\{\s*(\w+)\s*\(`),
		varRefPattern:   regexp.MustCompile(`\{\{\s*(\w+)\s*(?:\}\}|or\s)`),
	}
}

// Validate performs all validation checks on a Jinja2 template
func (v *TemplateValidator) Validate(template string) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]*ValidationError, 0),
		Warnings: make([]string, 0),
	}

	// Layer 1: Jinja2 syntax validation
	if err := v.validateJinja2Syntax(template); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
		return result // Stop early on syntax errors
	}

	// Layer 2: Function whitelist validation
	funcErrors := v.validateFunctions(template)
	if len(funcErrors) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, funcErrors...)
	}

	// Layer 2b: Variable whitelist validation
	varErrors := v.validateVariables(template)
	if len(varErrors) > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, varErrors...)
	}

	return result
}

// validateJinja2Syntax checks for basic Jinja2 syntax errors
func (v *TemplateValidator) validateJinja2Syntax(template string) *ValidationError {
	// Check for balanced {{ }}
	openDouble := strings.Count(template, "{{")
	closeDouble := strings.Count(template, "}}")
	if openDouble != closeDouble {
		return &ValidationError{
			Stage:      "Jinja2 语法检测",
			Message:    fmt.Sprintf("变量标签 {{ }} 不匹配: 打开 %d 个，关闭 %d 个", openDouble, closeDouble),
			Suggestion: "请检查所有 {{ 是否都有对应的 }}",
		}
	}

	// Check for balanced {% %}
	openBlock := strings.Count(template, "{%")
	closeBlock := strings.Count(template, "%}")
	if openBlock != closeBlock {
		return &ValidationError{
			Stage:      "Jinja2 语法检测",
			Message:    fmt.Sprintf("块标签 {%% %%} 不匹配: 打开 %d 个，关闭 %d 个", openBlock, closeBlock),
			Suggestion: "请检查所有 {% 是否都有对应的 %}",
		}
	}

	// Check for balanced for/endfor
	forPattern := regexp.MustCompile(`\{%\s*for\s+`)
	endforPattern := regexp.MustCompile(`\{%\s*endfor\s*%\}`)
	forCount := len(forPattern.FindAllString(template, -1))
	endforCount := len(endforPattern.FindAllString(template, -1))
	if forCount != endforCount {
		return &ValidationError{
			Stage:      "Jinja2 语法检测",
			Message:    fmt.Sprintf("for/endfor 不匹配: for %d 个，endfor %d 个", forCount, endforCount),
			Suggestion: "请检查每个 {% for %} 是否都有对应的 {% endfor %}",
		}
	}

	// Check for balanced if/endif
	ifPattern := regexp.MustCompile(`\{%\s*if\s+`)
	endifPattern := regexp.MustCompile(`\{%\s*endif\s*%\}`)
	ifCount := len(ifPattern.FindAllString(template, -1))
	endifCount := len(endifPattern.FindAllString(template, -1))
	if ifCount != endifCount {
		return &ValidationError{
			Stage:      "Jinja2 语法检测",
			Message:    fmt.Sprintf("if/endif 不匹配: if %d 个，endif %d 个", ifCount, endifCount),
			Suggestion: "请检查每个 {% if %} 是否都有对应的 {% endif %}",
		}
	}

	// Check for unsupported Jinja2 features
	unsupportedPatterns := map[string]string{
		`\{%\s*macro\s+`:        "macro 定义",
		`\{%\s*import\s+`:       "import 语句",
		`\{%\s*from\s+`:         "from 导入",
		`\{%\s*block\s+`:        "block 块",
		`\{%\s*extends\s+`:      "模板继承 (extends)",
		`\{%\s*include\s+`:      "include 包含",
		`\{%\s*set\s+`:          "set 变量赋值",
		`\{%\s*with\s+`:         "with 上下文",
		`\{\{\s*\w+\s*\|\s*\w+`: "过滤器 (|)",
	}

	for pattern, desc := range unsupportedPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindString(template); match != "" {
			return &ValidationError{
				Stage:      "Jinja2 语法检测",
				Message:    fmt.Sprintf("不支持的 Jinja2 特性: %s", desc),
				Suggestion: "请移除或改写该语法",
			}
		}
	}

	return nil
}

// validateFunctions checks all function calls against the whitelist
func (v *TemplateValidator) validateFunctions(template string) []*ValidationError {
	errors := make([]*ValidationError, 0)

	// Find all function calls
	matches := v.funcCallPattern.FindAllStringSubmatch(template, -1)
	usedFunctions := make(map[string]bool)

	for _, match := range matches {
		if len(match) >= 2 {
			funcName := match[1]
			usedFunctions[funcName] = true
		}
	}

	// Check against whitelist
	for funcName := range usedFunctions {
		if !v.allowedFunctions[funcName] {
			// Find line number
			lineNum := v.findLineNumber(template, funcName+"(")
			errors = append(errors, &ValidationError{
				Stage:      "函数白名单检测",
				Line:       lineNum,
				Message:    fmt.Sprintf("未定义的函数 '%s'", funcName),
				Suggestion: fmt.Sprintf("允许的函数: %s", v.getAllowedFunctionsString()),
			})
		}
	}

	return errors
}

// validateVariables checks all variable references against the whitelist
func (v *TemplateValidator) validateVariables(template string) []*ValidationError {
	errors := make([]*ValidationError, 0)

	// Find all variable references
	matches := v.varRefPattern.FindAllStringSubmatch(template, -1)
	usedVariables := make(map[string]bool)

	for _, match := range matches {
		if len(match) >= 2 {
			varName := match[1]
			// Skip if it's a function call
			if !v.allowedFunctions[varName] {
				usedVariables[varName] = true
			}
		}
	}

	// Check against whitelist
	for varName := range usedVariables {
		if !v.allowedVariables[varName] && !v.allowedFunctions[varName] {
			lineNum := v.findLineNumber(template, "{{ "+varName)
			errors = append(errors, &ValidationError{
				Stage:      "变量白名单检测",
				Line:       lineNum,
				Message:    fmt.Sprintf("未定义的变量 '%s'", varName),
				Suggestion: fmt.Sprintf("允许的变量: %s", v.getAllowedVariablesString()),
			})
		}
	}

	return errors
}

// findLineNumber finds the line number of a substring in the template
func (v *TemplateValidator) findLineNumber(template, substr string) int {
	idx := strings.Index(template, substr)
	if idx == -1 {
		return 0
	}
	return strings.Count(template[:idx], "\n") + 1
}

// getAllowedFunctionsString returns a comma-separated list of allowed functions
func (v *TemplateValidator) getAllowedFunctionsString() string {
	funcs := make([]string, 0, len(v.allowedFunctions))
	for f := range v.allowedFunctions {
		funcs = append(funcs, f)
	}
	return strings.Join(funcs, ", ")
}

// getAllowedVariablesString returns a comma-separated list of allowed variables
func (v *TemplateValidator) getAllowedVariablesString() string {
	vars := make([]string, 0, len(v.allowedVariables))
	for v := range v.allowedVariables {
		vars = append(vars, v)
	}
	return strings.Join(vars, ", ")
}

// AddAllowedFunction adds a function to the whitelist
func (v *TemplateValidator) AddAllowedFunction(funcName string) {
	v.allowedFunctions[funcName] = true
}

// AddAllowedVariable adds a variable to the whitelist
func (v *TemplateValidator) AddAllowedVariable(varName string) {
	v.allowedVariables[varName] = true
}

// ValidateFunctions is a standalone function for validating function whitelist
// This is exported for use in other packages
func ValidateFunctions(template string) error {
	validator := NewTemplateValidator()
	errors := validator.validateFunctions(template)
	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}
