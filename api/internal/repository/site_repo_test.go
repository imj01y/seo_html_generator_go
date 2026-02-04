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
