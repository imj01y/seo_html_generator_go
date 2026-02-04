// Package testing provides testing utilities for database operations.
// It includes mock database creation and test fixtures.
package testing

import "time"

// Fixtures 提供测试数据
type Fixtures struct{}

// NewFixtures creates a new fixtures instance
func NewFixtures() *Fixtures {
	return &Fixtures{}
}

// ValidSite returns a valid site for testing
func (f *Fixtures) ValidSite() map[string]interface{} {
	return map[string]interface{}{
		"id":               1,
		"site_group_id":    1,
		"domain":           "example.com",
		"name":             "Example Site",
		"template":         "default_template.html",
		"keyword_group_id": 1,
		"image_group_id":   1,
		"status":           1,
		"created_at":       time.Now(),
		"updated_at":       time.Now(),
	}
}

// ValidKeyword returns a valid keyword for testing
func (f *Fixtures) ValidKeyword() map[string]interface{} {
	return map[string]interface{}{
		"id":         uint(1),
		"keyword":    "测试关键词",
		"group_id":   1,
		"status":     1,
		"created_at": time.Now(),
	}
}

// ValidImage returns a valid image for testing
func (f *Fixtures) ValidImage() map[string]interface{} {
	return map[string]interface{}{
		"id":         uint(1),
		"url":        "https://example.com/image.jpg",
		"group_id":   1,
		"status":     1,
		"created_at": time.Now(),
	}
}
