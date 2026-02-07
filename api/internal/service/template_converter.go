package core

import (
	"regexp"
	"strings"
	"sync"
)

// TemplateConverter converts Jinja2 templates to Go text/template syntax
type TemplateConverter struct {
	rules []conversionRule
}

type conversionRule struct {
	pattern     *regexp.Regexp
	replacement string
}

// NewTemplateConverter creates a new template converter
func NewTemplateConverter() *TemplateConverter {
	tc := &TemplateConverter{}

	// Define conversion rules: Jinja2 -> Go text/template
	// Use $ prefix to reference top-level context (works inside range blocks)
	rules := []struct {
		pattern     string
		replacement string
	}{
		// Function calls without arguments
		{`\{\{\s*random_keyword\s*\(\s*\)\s*\}\}`, `{{$.RandomKeyword}}`},
		{`\{\{\s*random_hotspot\s*\(\s*\)\s*\}\}`, `{{$.RandomKeyword}}`},
		{`\{\{\s*keyword_with_emoji\s*\(\s*\)\s*\}\}`, `{{$.RandomKeywordEmoji}}`},
		{`\{\{\s*random_keyword_emoji\s*\(\s*\)\s*\}\}`, `{{$.RandomKeywordEmoji}}`},
		{`\{\{\s*random_url\s*\(\s*\)\s*\}\}`, `{{$.RandomURL}}`},
		{`\{\{\s*random_image\s*\(\s*\)\s*\}\}`, `{{$.RandomImage}}`},
		{`\{\{\s*content\s*\(\s*\)\s*\}\}`, `{{$.Content}}`},
		{`\{\{\s*content_with_pinyin\s*\(\s*\)\s*\}\}`, `{{$.Content}}`},
		{`\{\{\s*now\s*\(\s*\)\s*\}\}`, `{{$.Now}}`},

		// cls() function with argument - needs special handling
		// Use [^'"]* instead of [^'"]+ to allow empty strings like cls('')
		{`\{\{\s*cls\s*\(\s*['"]([^'"]*)['"]\s*\)\s*\}\}`, `{{$.Cls "${1}"}}`},
		{`\{\{\s*cls\s*\(\s*([^)]+)\s*\)\s*\}\}`, `{{$.Cls ${1}}}`},

		// encode() function
		{`\{\{\s*encode\s*\(\s*['"]([^'"]+)['"]\s*\)\s*\}\}`, `{{$.Encode "${1}"}}`},
		{`\{\{\s*encode_text\s*\(\s*['"]([^'"]+)['"]\s*\)\s*\}\}`, `{{$.Encode "${1}"}}`},

		// random_number(min, max) function
		{`\{\{\s*random_number\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)\s*\}\}`, `{{$.RandomNumber ${1} ${2}}}`},

		// Loop variable {{ i }} -> {{$i}}
		{`\{\{\s*i\s*\}\}`, `{{$$i}}`},

		// Simple variables
		{`\{\{\s*title\s*\}\}`, `{{$.Title}}`},
		{`\{\{\s*site_id\s*\}\}`, `{{$.SiteID}}`},
		{`\{\{\s*analytics_code\s*\}\}`, `{{$.AnalyticsCode}}`},
		{`\{\{\s*baidu_push_js\s*\}\}`, `{{$.BaiduPushJS}}`},
		{`\{\{\s*article_content\s*\}\}`, `{{$.ArticleContent}}`},

		// Variables with "or ''" fallback (Jinja2 default filter)
		// These are handled specially below to capitalize variable names

		// analytics_code or '' -> AnalyticsCode
		{`\{\{\s*analytics_code\s+or\s+['"]['"]?\s*\}\}`, `{{$.AnalyticsCode}}`},
		{`\{\{\s*baidu_push_js\s+or\s+['"]['"]?\s*\}\}`, `{{$.BaiduPushJS}}`},

		// For loops: {% for i in range(N) %} -> {{range $i := iterate N}}
		{`\{%\s*for\s+(\w+)\s+in\s+range\s*\(\s*(\d+)\s*\)\s*%\}`, `{{range $$${1} := iterate ${2}}}`},
		{`\{%\s*endfor\s*%\}`, `{{end}}`},

		// If statements
		{`\{%\s*if\s+([^%]+)\s*%\}`, `{{if ${1}}}`},
		{`\{%\s*elif\s+([^%]+)\s*%\}`, `{{else if ${1}}}`},
		{`\{%\s*else\s*%\}`, `{{else}}`},
		{`\{%\s*endif\s*%\}`, `{{end}}`},

		// Comments
		{`\{#[^#]*#\}`, ``},
	}

	for _, r := range rules {
		tc.rules = append(tc.rules, conversionRule{
			pattern:     regexp.MustCompile(r.pattern),
			replacement: r.replacement,
		})
	}

	return tc
}

// Convert converts a Jinja2 template to Go text/template syntax
func (tc *TemplateConverter) Convert(jinja2Template string) string {
	result := jinja2Template

	for _, rule := range tc.rules {
		result = rule.pattern.ReplaceAllString(result, rule.replacement)
	}

	// Handle remaining Jinja2 variable syntax {{ var }}
	// Convert to {{$.Var}} with capitalized first letter
	// Use $ to reference top-level context (works inside range blocks)
	// But skip Go template keywords
	goTemplateKeywords := map[string]bool{
		"end": true, "else": true, "if": true, "range": true,
		"with": true, "define": true, "template": true, "block": true,
		"nil": true, "true": true, "false": true,
	}
	remainingVarPattern := regexp.MustCompile(`\{\{\s*([a-z_][a-zA-Z0-9_]*)\s*\}\}`)
	result = remainingVarPattern.ReplaceAllStringFunc(result, func(match string) string {
		// Extract variable name
		submatches := remainingVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		varName := submatches[1]
		// Skip Go template keywords
		if goTemplateKeywords[varName] {
			return match
		}
		// Capitalize first letter
		if len(varName) > 0 {
			varName = strings.ToUpper(varName[:1]) + varName[1:]
		}
		return "{{$." + varName + "}}"
	})

	return result
}

// ConvertWithCache converts a template with caching
func (tc *TemplateConverter) ConvertWithCache(jinja2Template string, cacheKey string) string {
	// Check cache
	if cached, ok := templateConvertCache.Load(cacheKey); ok {
		return cached.(string)
	}

	// Convert
	result := tc.Convert(jinja2Template)

	// Store in cache
	templateConvertCache.Store(cacheKey, result)

	return result
}

// Global cache for converted templates
var templateConvertCache sync.Map

// Global converter instance
var globalConverter *TemplateConverter
var converterOnce sync.Once

// GetTemplateConverter returns the global template converter
func GetTemplateConverter() *TemplateConverter {
	converterOnce.Do(func() {
		globalConverter = NewTemplateConverter()
	})
	return globalConverter
}
