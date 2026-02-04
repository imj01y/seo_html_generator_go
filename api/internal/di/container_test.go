package di

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"seo-generator/api/pkg/config"
	testutil "seo-generator/api/internal/testing"
)

func TestContainer(t *testing.T) {
	db, _, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	cfg := &config.Config{
		Cache: config.CacheConfig{
			Dir: "/tmp/test-cache",
		},
	}

	container := NewContainer(db, cfg)

	t.Run("provides repositories", func(t *testing.T) {
		assert.NotNil(t, container.GetSiteRepository())
		assert.NotNil(t, container.GetKeywordRepository())
		assert.NotNil(t, container.GetImageRepository())
		assert.NotNil(t, container.GetArticleRepository())
		assert.NotNil(t, container.GetTitleRepository())
		assert.NotNil(t, container.GetContentRepository())
	})

	t.Run("singleton pattern", func(t *testing.T) {
		repo1 := container.GetSiteRepository()
		repo2 := container.GetSiteRepository()

		// 应该返回相同的实例
		assert.Same(t, repo1, repo2)
	})

	t.Run("provides database", func(t *testing.T) {
		assert.NotNil(t, container.GetDB())
		assert.Same(t, db, container.GetDB())
	})

	t.Run("provides config", func(t *testing.T) {
		assert.NotNil(t, container.GetConfig())
		assert.Same(t, cfg, container.GetConfig())
	})
}

func TestContainerWithRedis(t *testing.T) {
	db, _, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	cfg := &config.Config{}
	container := NewContainer(db, cfg)

	t.Run("redis is nil by default", func(t *testing.T) {
		assert.Nil(t, container.GetRedis())
	})
}

func TestContainerClose(t *testing.T) {
	db, _, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	cfg := &config.Config{}
	container := NewContainer(db, cfg)

	t.Run("close does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			container.Close()
		})
	})
}
