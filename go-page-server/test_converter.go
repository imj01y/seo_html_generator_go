// +build ignore

// Test script for template converter
package main

import (
	"fmt"
	"go-page-server/core"
)

func main() {
	converter := core.NewTemplateConverter()

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "random_keyword",
			input:  "{{ random_keyword() }}",
			expect: "{{.RandomKeyword}}",
		},
		{
			name:   "random_url",
			input:  "{{ random_url() }}",
			expect: "{{.RandomURL}}",
		},
		{
			name:   "random_image",
			input:  "{{ random_image() }}",
			expect: "{{.RandomImage}}",
		},
		{
			name:   "cls with string",
			input:  `{{ cls('header') }}`,
			expect: `{{.Cls "header"}}`,
		},
		{
			name:   "random_number",
			input:  "{{ random_number(1, 10) }}",
			expect: "{{.RandomNumber 1 10}}",
		},
		{
			name:   "now function",
			input:  "{{ now() }}",
			expect: "{{.Now}}",
		},
		{
			name:   "title variable",
			input:  "{{ title }}",
			expect: "{{.Title}}",
		},
		{
			name:   "site_id variable",
			input:  "{{ site_id }}",
			expect: "{{.SiteID}}",
		},
		{
			name:   "analytics_code or empty",
			input:  "{{ analytics_code or '' }}",
			expect: "{{.AnalyticsCode}}",
		},
		{
			name:   "for loop",
			input:  "{% for i in range(10) %}item{% endfor %}",
			expect: "{{range $i := iterate 10}}item{{end}}",
		},
		{
			name:   "content function",
			input:  "{{ content() }}",
			expect: "{{.Content}}",
		},
	}

	fmt.Println("Testing Template Converter")
	fmt.Println("==========================")

	passed := 0
	failed := 0

	for _, test := range tests {
		result := converter.Convert(test.input)
		if result == test.expect {
			fmt.Printf("[PASS] %s\n", test.name)
			passed++
		} else {
			fmt.Printf("[FAIL] %s\n", test.name)
			fmt.Printf("  Input:    %s\n", test.input)
			fmt.Printf("  Expected: %s\n", test.expect)
			fmt.Printf("  Got:      %s\n", result)
			failed++
		}
	}

	fmt.Println()
	fmt.Printf("Results: %d passed, %d failed\n", passed, failed)
}
