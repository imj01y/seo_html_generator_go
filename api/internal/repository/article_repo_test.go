// api/internal/repository/article_repo_test.go
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

func TestArticleRepository_GetByID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewArticleRepository(db)

	t.Run("success", func(t *testing.T) {
		expectedID := uint(1)
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "group_id", "title", "content", "status", "created_at", "updated_at"}).
			AddRow(expectedID, 1, "测试标题", "测试内容", 1, now, now)

		mock.ExpectQuery("SELECT (.+) FROM original_articles WHERE id = ?").
			WithArgs(expectedID).
			WillReturnRows(rows)

		article, err := repo.GetByID(context.Background(), expectedID)

		assert.NoError(t, err)
		assert.NotNil(t, article)
		assert.Equal(t, expectedID, article.ID)
		assert.Equal(t, "测试标题", article.Title)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM original_articles WHERE id = ?").
			WithArgs(uint(999)).
			WillReturnError(sql.ErrNoRows)

		article, err := repo.GetByID(context.Background(), 999)

		assert.Error(t, err)
		assert.Nil(t, article)
		assert.Equal(t, ErrNotFound, err)
	})
}

func TestArticleRepository_Create(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewArticleRepository(db)

	article := &models.OriginalArticle{
		GroupID: 1,
		Title:   "测试标题",
		Content: "测试内容",
		Status:  1,
	}

	mock.ExpectExec("INSERT INTO original_articles").
		WithArgs(article.GroupID, article.Title, article.Content, article.Status).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(context.Background(), article)

	assert.NoError(t, err)
	assert.Equal(t, uint(1), article.ID)
}

func TestArticleRepository_BatchImport(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewArticleRepository(db)

	articles := []*models.OriginalArticle{
		{GroupID: 1, Title: "文章1", Content: "内容1", Status: 1},
		{GroupID: 1, Title: "文章2", Content: "内容2", Status: 1},
	}

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO original_articles")
	mock.ExpectExec("INSERT INTO original_articles").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO original_articles").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	count, err := repo.BatchImport(context.Background(), articles)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
