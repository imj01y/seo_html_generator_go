//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"seo-generator/api/internal/repository"
)

func TestSiteRepository_Integration(t *testing.T) {
	ctx := getTestContext()
	repo := testContainer.GetSiteRepository()

	t.Run("list sites", func(t *testing.T) {
		filter := repository.SiteFilter{
			Pagination: repository.NewPagination(1, 10),
		}
		sites, total, err := repo.List(ctx, filter)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(0))
		t.Logf("Found %d sites (total: %d)", len(sites), total)
	})
}

func TestKeywordRepository_Integration(t *testing.T) {
	ctx := getTestContext()
	repo := testContainer.GetKeywordRepository()

	t.Run("count keywords", func(t *testing.T) {
		count, err := repo.CountByGroupID(ctx, 1)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(0))
		t.Logf("Group 1 has %d keywords", count)
	})

	t.Run("list keywords", func(t *testing.T) {
		status := 1
		filter := repository.KeywordFilter{
			Status:     &status,
			Pagination: repository.NewPagination(1, 10),
		}
		keywords, total, err := repo.List(ctx, filter)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(0))
		t.Logf("Found %d keywords (total: %d)", len(keywords), total)
	})
}

func TestImageRepository_Integration(t *testing.T) {
	ctx := getTestContext()
	repo := testContainer.GetImageRepository()

	t.Run("count images", func(t *testing.T) {
		count, err := repo.CountByGroupID(ctx, 1)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(0))
		t.Logf("Group 1 has %d images", count)
	})
}

func TestArticleRepository_Integration(t *testing.T) {
	ctx := getTestContext()
	repo := testContainer.GetArticleRepository()

	t.Run("count articles", func(t *testing.T) {
		count, err := repo.CountByGroupID(ctx, 1)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(0))
		t.Logf("Group 1 has %d articles", count)
	})
}
