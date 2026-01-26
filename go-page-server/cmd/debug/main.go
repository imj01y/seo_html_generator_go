package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"

	"go-page-server/core"
)

func iterate(n int) []int {
	result := make([]int, n)
	for i := 0; i < n; i++ {
		result[i] = i
	}
	return result
}

func main() {
	content, err := os.ReadFile("../../../database/templates/download_site.html")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	converter := core.NewTemplateConverter()
	goTemplate := converter.Convert(string(content))

	// Write converted template
	os.WriteFile("converted_go.txt", []byte(goTemplate), 0644)
	fmt.Printf("Converted template written to converted_go.txt (%d bytes)\n", len(goTemplate))

	// Try to parse
	funcMap := template.FuncMap{
		"iterate": iterate,
	}

	_, err = template.New("test").Funcs(funcMap).Parse(goTemplate)
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)

		// Show lines around error
		lines := strings.Split(goTemplate, "\n")
		fmt.Println("\n=== Last 10 lines ===")
		start := len(lines) - 10
		if start < 0 {
			start = 0
		}
		for i := start; i < len(lines); i++ {
			fmt.Printf("%d: %s\n", i+1, lines[i])
		}
	} else {
		fmt.Println("Template parsed successfully!")
	}
}
