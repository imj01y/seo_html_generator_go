// api/internal/testing/fixtures.go
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
		"template_id":      1,
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
		"id":       int64(1),
		"keyword":  "测试关键词",
		"group_id": 1,
		"status":   1,
	}
}

// ValidImage returns a valid image for testing
func (f *Fixtures) ValidImage() map[string]interface{} {
	return map[string]interface{}{
		"id":       int64(1),
		"url":      "https://example.com/image.jpg",
		"group_id": 1,
		"status":   1,
	}
}
