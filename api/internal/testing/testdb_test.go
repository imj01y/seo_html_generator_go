// Package testing provides testing utilities for database operations.
// It includes mock database creation and test fixtures.
package testing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMockDB(t *testing.T) {
	db, mock, cleanup := NewMockDB(t)
	defer cleanup()

	assert.NotNil(t, db)
	assert.NotNil(t, mock)
}

func TestFixtures(t *testing.T) {
	fixtures := NewFixtures()

	site := fixtures.ValidSite()
	assert.Equal(t, 1, site["id"])
	assert.Equal(t, "example.com", site["domain"])
	assert.Equal(t, "Example Site", site["name"])
	assert.Equal(t, "default_template.html", site["template"])

	keyword := fixtures.ValidKeyword()
	assert.Equal(t, uint(1), keyword["id"])
	assert.NotNil(t, keyword["created_at"])

	image := fixtures.ValidImage()
	assert.Equal(t, uint(1), image["id"])
	assert.NotNil(t, image["created_at"])
}
