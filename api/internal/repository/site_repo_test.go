// api/internal/repository/site_repo_test.go
package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	testutil "seo-generator/api/internal/testing"

	models "seo-generator/api/internal/model"
)

func TestSiteRepository_GetByID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewSiteRepository(db)

	t.Run("success", func(t *testing.T) {
		expectedID := 1
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "site_group_id", "domain", "name", "template", "status", "created_at", "updated_at"}).
			AddRow(expectedID, 1, "example.com", "示例站点", "default", 1, now, now)

		mock.ExpectQuery("SELECT (.+) FROM sites WHERE id = ?").
			WithArgs(expectedID).
			WillReturnRows(rows)

		site, err := repo.GetByID(context.Background(), expectedID)

		assert.NoError(t, err)
		assert.NotNil(t, site)
		assert.Equal(t, expectedID, site.ID)
		assert.Equal(t, "example.com", site.Domain)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM sites WHERE id = ?").
			WithArgs(999).
			WillReturnError(sql.ErrNoRows)

		site, err := repo.GetByID(context.Background(), 999)

		assert.Error(t, err)
		assert.Nil(t, site)
		assert.Equal(t, ErrNotFound, err)
	})
}

func TestSiteRepository_GetByDomain(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewSiteRepository(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "site_group_id", "domain", "name", "template", "status", "created_at", "updated_at"}).
		AddRow(1, 1, "example.com", "示例站点", "default", 1, now, now)

	mock.ExpectQuery("SELECT (.+) FROM sites WHERE domain = ?").
		WithArgs("example.com").
		WillReturnRows(rows)

	site, err := repo.GetByDomain(context.Background(), "example.com")

	assert.NoError(t, err)
	assert.NotNil(t, site)
	assert.Equal(t, "example.com", site.Domain)
}

func TestSiteRepository_List(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewSiteRepository(db)

	t.Run("success with all filters and pagination", func(t *testing.T) {
		siteGroupID := 1
		status := 1
		filter := SiteFilter{
			SiteGroupID: &siteGroupID,
			Status:      &status,
			Pagination:  NewPagination(1, 10),
		}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(30)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM sites WHERE site_group_id = \\? AND status = \\?").
			WithArgs(siteGroupID, status).
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "site_group_id", "domain", "name", "template", "status",
			"keyword_group_id", "image_group_id", "article_group_id",
			"icp_number", "baidu_token", "analytics", "created_at", "updated_at"}).
			AddRow(1, 1, "example1.com", "站点1", "default", 1, nil, nil, nil, nil, nil, nil, now, now).
			AddRow(2, 1, "example2.com", "站点2", "default", 1, nil, nil, nil, nil, nil, nil, now, now)

		mock.ExpectQuery("SELECT (.+) FROM sites WHERE site_group_id = \\? AND status = \\? ORDER BY id DESC LIMIT 10 OFFSET 0").
			WithArgs(siteGroupID, status).
			WillReturnRows(rows)

		sites, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(30), total)
		assert.Len(t, sites, 2)
		assert.Equal(t, "example1.com", sites[0].Domain)
		assert.Equal(t, "example2.com", sites[1].Domain)
	})

	t.Run("success with site group filter only", func(t *testing.T) {
		siteGroupID := 1
		filter := SiteFilter{
			SiteGroupID: &siteGroupID,
		}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(5)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM sites WHERE site_group_id = \\?").
			WithArgs(siteGroupID).
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "site_group_id", "domain", "name", "template", "status",
			"keyword_group_id", "image_group_id", "article_group_id",
			"icp_number", "baidu_token", "analytics", "created_at", "updated_at"}).
			AddRow(1, 1, "example.com", "站点", "default", 1, nil, nil, nil, nil, nil, nil, now, now)

		mock.ExpectQuery("SELECT (.+) FROM sites WHERE site_group_id = \\? ORDER BY id DESC").
			WithArgs(siteGroupID).
			WillReturnRows(rows)

		sites, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(5), total)
		assert.Len(t, sites, 1)
	})

	t.Run("success with domain filter", func(t *testing.T) {
		domain := "example"
		filter := SiteFilter{
			Domain: &domain,
		}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(3)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM sites WHERE domain LIKE \\?").
			WithArgs("%example%").
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "site_group_id", "domain", "name", "template", "status",
			"keyword_group_id", "image_group_id", "article_group_id",
			"icp_number", "baidu_token", "analytics", "created_at", "updated_at"}).
			AddRow(1, 1, "example.com", "站点", "default", 1, nil, nil, nil, nil, nil, nil, now, now)

		mock.ExpectQuery("SELECT (.+) FROM sites WHERE domain LIKE \\? ORDER BY id DESC").
			WithArgs("%example%").
			WillReturnRows(rows)

		sites, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, sites, 1)
	})

	t.Run("success with status filter only", func(t *testing.T) {
		status := 1
		filter := SiteFilter{
			Status: &status,
		}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(15)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM sites WHERE status = \\?").
			WithArgs(status).
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "site_group_id", "domain", "name", "template", "status",
			"keyword_group_id", "image_group_id", "article_group_id",
			"icp_number", "baidu_token", "analytics", "created_at", "updated_at"}).
			AddRow(1, 1, "example.com", "站点", "default", 1, nil, nil, nil, nil, nil, nil, now, now)

		mock.ExpectQuery("SELECT (.+) FROM sites WHERE status = \\? ORDER BY id DESC").
			WithArgs(status).
			WillReturnRows(rows)

		sites, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(15), total)
		assert.Len(t, sites, 1)
	})

	t.Run("success without filters", func(t *testing.T) {
		filter := SiteFilter{}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(50)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM sites").
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "site_group_id", "domain", "name", "template", "status",
			"keyword_group_id", "image_group_id", "article_group_id",
			"icp_number", "baidu_token", "analytics", "created_at", "updated_at"}).
			AddRow(1, 1, "example1.com", "站点1", "default", 1, nil, nil, nil, nil, nil, nil, now, now).
			AddRow(2, 2, "example2.com", "站点2", "default", 1, nil, nil, nil, nil, nil, nil, now, now)

		mock.ExpectQuery("SELECT (.+) FROM sites ORDER BY id DESC").
			WillReturnRows(rows)

		sites, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(50), total)
		assert.Len(t, sites, 2)
	})
}

func TestSiteRepository_Create(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewSiteRepository(db)

	site := &models.Site{
		SiteGroupID: 1,
		Domain:      "example.com",
		Name:        "示例站点",
		Template:    "default",
		Status:      1,
	}

	mock.ExpectExec("INSERT INTO sites").
		WithArgs(site.SiteGroupID, site.Domain, site.Name, site.Template, site.Status).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(context.Background(), site)

	assert.NoError(t, err)
	assert.Equal(t, 1, site.ID)
}
