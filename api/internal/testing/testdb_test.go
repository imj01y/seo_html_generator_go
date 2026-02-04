// api/internal/testing/testdb_test.go
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

	keyword := fixtures.ValidKeyword()
	assert.Equal(t, int64(1), keyword["id"])

	image := fixtures.ValidImage()
	assert.Equal(t, int64(1), image["id"])
}
