package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Jinja2ToQuickTemplate converts Jinja2 template syntax to quicktemplate syntax
type Jinja2ToQuickTemplate struct {
	// Template name for generating struct name
	templateName string

	// Patterns for matching Jinja2 syntax
	varPattern     *regexp.Regexp // {{ var }}
	funcPattern    *regexp.Regexp // {{ func() }}
	forPattern     *regexp.Regexp // {% for i in range(n) %}
	endforPattern  *regexp.Regexp // {% endfor %}
	ifPattern      *regexp.Regexp // {% if condition %}
	elifPattern    *regexp.Regexp // {% elif condition %}
	elsePattern    *regexp.Regexp // {% else %}
	endifPattern   *regexp.Regexp // {% endif %}
	commentPattern *regexp.Regexp // {# comment #}

	// Function patterns
	clsPattern          *regexp.Regexp // cls('name')
	randomURLPattern    *regexp.Regexp // random_url()
	randomKeywordPattern *regexp.Regexp // random_keyword() / random_hotspot() / keyword_with_emoji()
	randomImagePattern  *regexp.Regexp // random_image()
	randomNumberPattern *regexp.Regexp // random_number(min, max)
	encodePattern       *regexp.Regexp // encode('text') / encode_text('text')
	nowPattern          *regexp.Regexp // now()
	contentPattern      *regexp.Regexp // content() / content_with_pinyin()

	// Variable patterns with 'or' default
	varOrPattern *regexp.Regexp // {{ var or 'default' }}
}

// NewJinja2ToQuickTemplate creates a new converter
func NewJinja2ToQuickTemplate(templateName string) *Jinja2ToQuickTemplate {
	return &Jinja2ToQuickTemplate{
		templateName: templateName,

		// General patterns
		varPattern:     regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`),
		varOrPattern:   regexp.MustCompile(`\{\{\s*(\w+)\s+or\s+['"]([^'"]*)['"]\s*\}\}`),
		funcPattern:    regexp.MustCompile(`\{\{\s*(\w+)\s*\(\s*\)\s*\}\}`),
		forPattern:     regexp.MustCompile(`\{%\s*for\s+(\w+)\s+in\s+range\s*\(\s*(\d+)\s*\)\s*%\}`),
		endforPattern:  regexp.MustCompile(`\{%\s*endfor\s*%\}`),
		ifPattern:      regexp.MustCompile(`\{%\s*if\s+(.+?)\s*%\}`),
		elifPattern:    regexp.MustCompile(`\{%\s*elif\s+(.+?)\s*%\}`),
		elsePattern:    regexp.MustCompile(`\{%\s*else\s*%\}`),
		endifPattern:   regexp.MustCompile(`\{%\s*endif\s*%\}`),
		commentPattern: regexp.MustCompile(`\{#.*?#\}`),

		// Function patterns
		clsPattern:          regexp.MustCompile(`\{\{\s*cls\s*\(\s*['"]([^'"]*)['"]\s*\)\s*\}\}`),
		randomURLPattern:    regexp.MustCompile(`\{\{\s*random_url\s*\(\s*\)\s*\}\}`),
		randomKeywordPattern: regexp.MustCompile(`\{\{\s*(random_keyword|random_hotspot|keyword_with_emoji)\s*\(\s*\)\s*\}\}`),
		randomImagePattern:  regexp.MustCompile(`\{\{\s*random_image\s*\(\s*\)\s*\}\}`),
		randomNumberPattern: regexp.MustCompile(`\{\{\s*random_number\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)\s*\}\}`),
		encodePattern:       regexp.MustCompile(`\{\{\s*(encode|encode_text)\s*\(\s*['"]([^'"]*)['"]\s*\)\s*\}\}`),
		nowPattern:          regexp.MustCompile(`\{\{\s*now\s*\(\s*\)\s*\}\}`),
		contentPattern:      regexp.MustCompile(`\{\{\s*(content|content_with_pinyin)\s*\(\s*\)\s*\}\}`),
	}
}

// Convert converts Jinja2 template to quicktemplate format
func (c *Jinja2ToQuickTemplate) Convert(jinja2Template string) (string, error) {
	result := jinja2Template

	// Step 1: Remove comments
	result = c.commentPattern.ReplaceAllString(result, "")

	// Step 2: Convert functions (must be before variable conversion)
	result = c.convertFunctions(result)

	// Step 3: Convert control structures
	result = c.convertControlStructures(result)

	// Step 4: Convert variables with 'or' default
	result = c.convertVariablesWithDefault(result)

	// Step 5: Convert remaining simple variables
	result = c.convertSimpleVariables(result)

	// Step 6: Wrap with quicktemplate header and func
	result = c.wrapTemplate(result)

	return result, nil
}

// convertFunctions converts Jinja2 function calls to quicktemplate
func (c *Jinja2ToQuickTemplate) convertFunctions(template string) string {
	result := template

	// cls('name') -> {%s p.Cls("name") %}
	result = c.clsPattern.ReplaceAllString(result, `{%s p.Cls("$1") %}`)

	// random_url() -> {%s p.RandomURL() %}
	result = c.randomURLPattern.ReplaceAllString(result, `{%s p.RandomURL() %}`)

	// random_keyword() / random_hotspot() / keyword_with_emoji() -> {%s= string(p.RandomKeyword()) %}
	result = c.randomKeywordPattern.ReplaceAllString(result, `{%s= string(p.RandomKeyword()) %}`)

	// random_image() -> {%s p.RandomImage() %}
	result = c.randomImagePattern.ReplaceAllString(result, `{%s p.RandomImage() %}`)

	// random_number(min, max) -> {%d p.RandomNumber(min, max) %}
	result = c.randomNumberPattern.ReplaceAllString(result, `{%d p.RandomNumber($1, $2) %}`)

	// encode('text') / encode_text('text') -> {%s= p.Encode("$2") %}
	result = c.encodePattern.ReplaceAllString(result, `{%s= p.Encode("$2") %}`)

	// now() -> {%s p.Now() %}
	result = c.nowPattern.ReplaceAllString(result, `{%s p.Now() %}`)

	// content() / content_with_pinyin() -> {%s= p.Content %}
	result = c.contentPattern.ReplaceAllString(result, `{%s= p.Content %}`)

	return result
}

// convertControlStructures converts Jinja2 control structures
func (c *Jinja2ToQuickTemplate) convertControlStructures(template string) string {
	result := template

	// {% for i in range(n) %} -> {% for i := 0; i < n; i++ %}
	result = c.forPattern.ReplaceAllStringFunc(result, func(match string) string {
		submatches := c.forPattern.FindStringSubmatch(match)
		if len(submatches) == 3 {
			varName := submatches[1]
			count := submatches[2]
			return fmt.Sprintf("{%% for %s := 0; %s < %s; %s++ %%}", varName, varName, count, varName)
		}
		return match
	})

	// {% endfor %} -> {% endfor %}
	result = c.endforPattern.ReplaceAllString(result, `{% endfor %}`)

	// {% if condition %} -> {% if condition %}
	result = c.ifPattern.ReplaceAllString(result, `{% if $1 %}`)

	// {% elif condition %} -> {% elseif condition %}
	result = c.elifPattern.ReplaceAllString(result, `{% elseif $1 %}`)

	// {% else %} -> {% else %}
	result = c.elsePattern.ReplaceAllString(result, `{% else %}`)

	// {% endif %} -> {% endif %}
	result = c.endifPattern.ReplaceAllString(result, `{% endif %}`)

	return result
}

// convertVariablesWithDefault converts variables with 'or' default values
func (c *Jinja2ToQuickTemplate) convertVariablesWithDefault(template string) string {
	return c.varOrPattern.ReplaceAllStringFunc(template, func(match string) string {
		submatches := c.varOrPattern.FindStringSubmatch(match)
		if len(submatches) == 3 {
			varName := submatches[1]
			// Convert to quicktemplate syntax based on variable type
			switch varName {
			case "analytics_code", "baidu_push_js", "article_content":
				// These are HTML safe, use raw output
				return fmt.Sprintf("{%%s= p.%s %%}", toPascalCase(varName))
			default:
				return fmt.Sprintf("{%%s p.%s %%}", toPascalCase(varName))
			}
		}
		return match
	})
}

// convertSimpleVariables converts simple variable references
func (c *Jinja2ToQuickTemplate) convertSimpleVariables(template string) string {
	return c.varPattern.ReplaceAllStringFunc(template, func(match string) string {
		submatches := c.varPattern.FindStringSubmatch(match)
		if len(submatches) == 2 {
			varName := submatches[1]

			// Skip if already converted (contains p. or %)
			if strings.Contains(match, "p.") || strings.Contains(match, "%") {
				return match
			}

			// Convert based on variable type
			switch varName {
			case "title":
				return `{%s p.Title %}`
			case "site_id":
				return `{%d p.SiteID %}`
			case "analytics_code":
				return `{%s= p.AnalyticsCode %}`
			case "baidu_push_js":
				return `{%s= p.BaiduPushJS %}`
			case "article_content":
				return `{%s= p.ArticleContent %}`
			case "i": // Loop variable
				return `{%d i %}`
			default:
				// Unknown variable, assume string
				return fmt.Sprintf("{%%s p.%s %%}", toPascalCase(varName))
			}
		}
		return match
	})
}

// wrapTemplate wraps the converted template with quicktemplate header
func (c *Jinja2ToQuickTemplate) wrapTemplate(template string) string {
	header := `{% import "html/template" %}

{% code
// PageParams holds all parameters needed for page rendering
type PageParams struct {
    Title          string
    SiteID         int
    AnalyticsCode  string
    BaiduPushJS    string
    ArticleContent string
    Content        string

    // Function providers
    clsFunc      func(string) string
    urlFunc      func() string
    keywordFunc  func() string
    imageFunc    func() string
    numberFunc   func(int, int) int
    nowFunc      func() string
}

// NewPageParams creates a new PageParams with function providers
func NewPageParams(
    title string,
    siteID int,
    analyticsCode, baiduPushJS, articleContent, content string,
    clsFunc func(string) string,
    urlFunc func() string,
    keywordFunc func() string,
    imageFunc func() string,
    numberFunc func(int, int) int,
    nowFunc func() string,
) *PageParams {
    return &PageParams{
        Title:          title,
        SiteID:         siteID,
        AnalyticsCode:  analyticsCode,
        BaiduPushJS:    baiduPushJS,
        ArticleContent: articleContent,
        Content:        content,
        clsFunc:        clsFunc,
        urlFunc:        urlFunc,
        keywordFunc:    keywordFunc,
        imageFunc:      imageFunc,
        numberFunc:     numberFunc,
        nowFunc:        nowFunc,
    }
}

// Cls generates random CSS class
func (p *PageParams) Cls(name string) string {
    if p.clsFunc != nil {
        return p.clsFunc(name)
    }
    return name
}

// RandomURL returns a random URL
func (p *PageParams) RandomURL() string {
    if p.urlFunc != nil {
        return p.urlFunc()
    }
    return "/?123456.html"
}

// RandomKeyword returns a random keyword (HTML safe)
func (p *PageParams) RandomKeyword() template.HTML {
    if p.keywordFunc != nil {
        return template.HTML(p.keywordFunc())
    }
    return template.HTML("关键词")
}

// RandomImage returns a random image URL
func (p *PageParams) RandomImage() string {
    if p.imageFunc != nil {
        return p.imageFunc()
    }
    return "/static/images/default.png"
}

// RandomNumber returns a random number in range [min, max]
func (p *PageParams) RandomNumber(min, max int) int {
    if p.numberFunc != nil {
        return p.numberFunc(min, max)
    }
    return min
}

// Now returns current time string
func (p *PageParams) Now() string {
    if p.nowFunc != nil {
        return p.nowFunc()
    }
    return "2024-01-01 00:00:00"
}

// Encode encodes text as HTML entities
func (p *PageParams) Encode(text string) template.HTML {
    // TODO: Implement HTML entity encoding
    return template.HTML(text)
}
%}

{% func (p *PageParams) Render() %}
`

	footer := `
{% endfunc %}
`

	return header + template + footer
}

// toPascalCase converts snake_case to PascalCase
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// ConvertResult contains the conversion result
type ConvertResult struct {
	QuickTemplate string   // Converted quicktemplate content
	Warnings      []string // Non-fatal warnings during conversion
	Errors        []string // Fatal errors that prevent conversion
}

// ConvertWithValidation converts and validates the template
func (c *Jinja2ToQuickTemplate) ConvertWithValidation(jinja2Template string) *ConvertResult {
	result := &ConvertResult{
		Warnings: make([]string, 0),
		Errors:   make([]string, 0),
	}

	// Check for unsupported features
	unsupportedPatterns := map[string]string{
		`\{%\s*macro\s+`:       "macro 定义",
		`\{%\s*import\s+`:      "import 语句",
		`\{%\s*from\s+`:        "from 导入",
		`\{%\s*block\s+`:       "block 块",
		`\{%\s*extends\s+`:     "模板继承",
		`\{%\s*include\s+`:     "include 包含",
		`\{%\s*set\s+`:         "set 变量赋值",
		`\{%\s*with\s+`:        "with 上下文",
		`\{\{\s*\w+\s*\|\s*\w+`: "过滤器",
	}

	for pattern, desc := range unsupportedPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(jinja2Template) {
			result.Errors = append(result.Errors, fmt.Sprintf("不支持的 Jinja2 特性: %s", desc))
		}
	}

	if len(result.Errors) > 0 {
		return result
	}

	// Perform conversion
	converted, err := c.Convert(jinja2Template)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	result.QuickTemplate = converted

	// Check for potential issues
	// Look for unconverted Jinja2 syntax
	remainingJinja2 := regexp.MustCompile(`\{\{[^%].*?[^%]\}\}|\{%[^}]*%\}`)
	matches := remainingJinja2.FindAllString(converted, -1)
	for _, match := range matches {
		// Skip valid quicktemplate syntax
		if strings.HasPrefix(match, "{%s") || strings.HasPrefix(match, "{%d") ||
			strings.HasPrefix(match, "{%f") || strings.HasPrefix(match, "{%v") ||
			strings.HasPrefix(match, "{%=") || strings.HasPrefix(match, "{% ") {
			continue
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("可能未转换的语法: %s", match))
	}

	return result
}

// ParseRangeValue parses a range value that could be a number or expression
func ParseRangeValue(s string) (int, error) {
	// Try to parse as integer
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("无法解析范围值: %s", s)
	}
	return n, nil
}

// ValidateJinja2Syntax performs basic Jinja2 syntax validation
func ValidateJinja2Syntax(template string) error {
	// Check for balanced {{ }}
	openDouble := strings.Count(template, "{{")
	closeDouble := strings.Count(template, "}}")
	if openDouble != closeDouble {
		return fmt.Errorf("变量标签 {{ }} 不匹配: 打开 %d 个，关闭 %d 个", openDouble, closeDouble)
	}

	// Check for balanced {% %}
	openBlock := strings.Count(template, "{%")
	closeBlock := strings.Count(template, "%}")
	if openBlock != closeBlock {
		return fmt.Errorf("块标签 {%% %%} 不匹配: 打开 %d 个，关闭 %d 个", openBlock, closeBlock)
	}

	// Check for balanced for/endfor
	forCount := len(regexp.MustCompile(`\{%\s*for\s+`).FindAllString(template, -1))
	endforCount := len(regexp.MustCompile(`\{%\s*endfor\s*%\}`).FindAllString(template, -1))
	if forCount != endforCount {
		return fmt.Errorf("for/endfor 不匹配: for %d 个，endfor %d 个", forCount, endforCount)
	}

	// Check for balanced if/endif
	ifCount := len(regexp.MustCompile(`\{%\s*if\s+`).FindAllString(template, -1))
	endifCount := len(regexp.MustCompile(`\{%\s*endif\s*%\}`).FindAllString(template, -1))
	if ifCount != endifCount {
		return fmt.Errorf("if/endif 不匹配: if %d 个，endif %d 个", ifCount, endifCount)
	}

	return nil
}
