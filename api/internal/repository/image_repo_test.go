// api/internal/repository/image_repo_test.go
package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	testutil "seo-generator/api/internal/testing"

	models "seo-generator/api/internal/model"
)

func TestImageRepository_GetByID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	t.Run("success", func(t *testing.T) {
		expectedID := uint(1)
		rows := sqlmock.NewRows([]string{"id", "url", "group_id", "status"}).
			AddRow(expectedID, "https://example.com/image.jpg", 1, 1)

		mock.ExpectQuery("SELECT (.+) FROM images WHERE id = ?").
			WithArgs(expectedID).
			WillReturnRows(rows)

		image, err := repo.GetByID(context.Background(), expectedID)

		assert.NoError(t, err)
		assert.NotNil(t, image)
		assert.Equal(t, expectedID, image.ID)
		assert.Equal(t, "https://example.com/image.jpg", image.URL)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM images WHERE id = ?").
			WithArgs(uint(999)).
			WillReturnError(sql.ErrNoRows)

		image, err := repo.GetByID(context.Background(), 999)

		assert.Error(t, err)
		assert.Nil(t, image)
		assert.Equal(t, ErrNotFound, err)
	})
}

func TestImageRepository_RandomByGroupID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	rows := sqlmock.NewRows([]string{"id", "url", "group_id", "status"}).
		AddRow(uint(1), "https://example.com/image1.jpg", 1, 1).
		AddRow(uint(2), "https://example.com/image2.jpg", 1, 1)

	mock.ExpectQuery("SELECT (.+) FROM images WHERE group_id = (.+) ORDER BY RAND\\(\\) LIMIT (.+)").
		WithArgs(1, 10).
		WillReturnRows(rows)

	images, err := repo.RandomByGroupID(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Equal(t, "https://example.com/image1.jpg", images[0].URL)
}

func TestImageRepository_BatchImport(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	images := []*models.Image{
		{URL: "https://example.com/image1.jpg", GroupID: 1, Status: 1},
		{URL: "https://example.com/image2.jpg", GroupID: 1, Status: 1},
	}

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO images")
	mock.ExpectExec("INSERT INTO images").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO images").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	count, err := repo.BatchImport(context.Background(), images)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
