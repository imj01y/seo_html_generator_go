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

func TestArticleRepository_List(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewArticleRepository(db)

	t.Run("success with filter and pagination", func(t *testing.T) {
		groupID := 1
		filter := ArticleFilter{
			GroupID:    &groupID,
			Pagination: NewPagination(1, 10),
		}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(25)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM original_articles WHERE group_id = \\?").
			WithArgs(groupID).
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "group_id", "source_id", "source_url", "title", "content", "status", "created_at", "updated_at"}).
			AddRow(uint(1), 1, nil, nil, "文章1", "内容1", 1, now, now).
			AddRow(uint(2), 1, nil, nil, "文章2", "内容2", 1, now, now)

		mock.ExpectQuery("SELECT (.+) FROM original_articles WHERE group_id = \\? ORDER BY id DESC LIMIT \\? OFFSET \\?").
			WithArgs(groupID, 10, 0).
			WillReturnRows(rows)

		articles, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(25), total)
		assert.Len(t, articles, 2)
		assert.Equal(t, "文章1", articles[0].Title)
		assert.Equal(t, "文章2", articles[1].Title)
	})

	t.Run("success with page 2", func(t *testing.T) {
		groupID := 1
		filter := ArticleFilter{
			GroupID:    &groupID,
			Pagination: NewPagination(2, 10),
		}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(25)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM original_articles WHERE group_id = \\?").
			WithArgs(groupID).
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "group_id", "source_id", "source_url", "title", "content", "status", "created_at", "updated_at"}).
			AddRow(uint(11), 1, nil, nil, "文章11", "内容11", 1, now, now)

		mock.ExpectQuery("SELECT (.+) FROM original_articles WHERE group_id = \\? ORDER BY id DESC LIMIT \\? OFFSET \\?").
			WithArgs(groupID, 10, 10).
			WillReturnRows(rows)

		articles, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(25), total)
		assert.Len(t, articles, 1)
		assert.Equal(t, "文章11", articles[0].Title)
	})

	t.Run("success without filters", func(t *testing.T) {
		filter := ArticleFilter{}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(100)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM original_articles").
			WillReturnRows(countRows)

		// Mock list query
		now := time.Now()
		rows := sqlmock.NewRows([]string{"id", "group_id", "source_id", "source_url", "title", "content", "status", "created_at", "updated_at"}).
			AddRow(uint(1), 1, nil, nil, "文章1", "内容1", 1, now, now).
			AddRow(uint(2), 2, nil, nil, "文章2", "内容2", 1, now, now)

		mock.ExpectQuery("SELECT (.+) FROM original_articles ORDER BY id DESC LIMIT \\? OFFSET \\?").
			WithArgs(10, 0).
			WillReturnRows(rows)

		articles, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(100), total)
		assert.Len(t, articles, 2)
	})

	t.Run("empty result", func(t *testing.T) {
		groupID := 999
		filter := ArticleFilter{
			GroupID: &groupID,
		}

		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM original_articles WHERE group_id = \\?").
			WithArgs(groupID).
			WillReturnRows(countRows)

		// Mock list query
		rows := sqlmock.NewRows([]string{"id", "group_id", "source_id", "source_url", "title", "content", "status", "created_at", "updated_at"})

		mock.ExpectQuery("SELECT (.+) FROM original_articles WHERE group_id = \\? ORDER BY id DESC LIMIT \\? OFFSET \\?").
			WithArgs(groupID, 10, 0).
			WillReturnRows(rows)

		articles, total, err := repo.List(context.Background(), filter)

		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Len(t, articles, 0)
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

func TestArticleRepository_Update(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewArticleRepository(db)

	t.Run("success", func(t *testing.T) {
		article := &models.OriginalArticle{
			ID:      1,
			GroupID: 1,
			Title:   "更新标题",
			Content: "更新内容",
			Status:  1,
		}

		mock.ExpectExec("UPDATE original_articles SET").
			WithArgs(article.GroupID, article.SourceID, article.SourceURL,
				article.Title, article.Content, article.Status, article.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.Update(context.Background(), article)

		assert.NoError(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		article := &models.OriginalArticle{
			ID:      999,
			GroupID: 1,
			Title:   "标题",
			Content: "内容",
			Status:  1,
		}

		mock.ExpectExec("UPDATE original_articles SET").
			WithArgs(article.GroupID, article.SourceID, article.SourceURL,
				article.Title, article.Content, article.Status, article.ID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.Update(context.Background(), article)

		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("database error", func(t *testing.T) {
		article := &models.OriginalArticle{
			ID:      1,
			GroupID: 1,
			Title:   "标题",
			Content: "内容",
			Status:  1,
		}

		mock.ExpectExec("UPDATE original_articles SET").
			WithArgs(article.GroupID, article.SourceID, article.SourceURL,
				article.Title, article.Content, article.Status, article.ID).
			WillReturnError(sql.ErrConnDone)

		err := repo.Update(context.Background(), article)

		assert.Error(t, err)
	})
}

func TestArticleRepository_Delete(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewArticleRepository(db)

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM original_articles WHERE id = \\?").
			WithArgs(uint(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.Delete(context.Background(), 1)

		assert.NoError(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM original_articles WHERE id = \\?").
			WithArgs(uint(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.Delete(context.Background(), 999)

		assert.Equal(t, ErrNotFound, err)
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM original_articles WHERE id = \\?").
			WithArgs(uint(1)).
			WillReturnError(sql.ErrConnDone)

		err := repo.Delete(context.Background(), 1)

		assert.Error(t, err)
	})
}

func TestArticleRepository_CountByGroupID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewArticleRepository(db)

	t.Run("success", func(t *testing.T) {
		groupID := 1
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(50)
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM original_articles WHERE group_id = \\?").
			WithArgs(groupID).
			WillReturnRows(countRows)

		count, err := repo.CountByGroupID(context.Background(), groupID)

		assert.NoError(t, err)
		assert.Equal(t, int64(50), count)
	})

	t.Run("database error", func(t *testing.T) {
		groupID := 1
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM original_articles WHERE group_id = \\?").
			WithArgs(groupID).
			WillReturnError(sql.ErrConnDone)

		count, err := repo.CountByGroupID(context.Background(), groupID)

		assert.Error(t, err)
		assert.Equal(t, int64(0), count)
	})
}
