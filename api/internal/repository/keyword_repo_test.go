// api/internal/repository/keyword_repo_test.go
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

func TestKeywordRepository_GetByID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewKeywordRepository(db)

	t.Run("success", func(t *testing.T) {
		expectedID := uint(1)
		rows := sqlmock.NewRows([]string{"id", "keyword", "group_id", "status"}).
			AddRow(expectedID, "测试关键词", 1, 1)

		mock.ExpectQuery("SELECT (.+) FROM keywords WHERE id = ?").
			WithArgs(expectedID).
			WillReturnRows(rows)

		keyword, err := repo.GetByID(context.Background(), expectedID)

		assert.NoError(t, err)
		assert.NotNil(t, keyword)
		assert.Equal(t, expectedID, keyword.ID)
		assert.Equal(t, "测试关键词", keyword.Keyword)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM keywords WHERE id = ?").
			WithArgs(uint(999)).
			WillReturnError(sql.ErrNoRows)

		keyword, err := repo.GetByID(context.Background(), 999)

		assert.Error(t, err)
		assert.Nil(t, keyword)
		assert.Equal(t, ErrNotFound, err)
	})
}

func TestKeywordRepository_RandomByGroupID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewKeywordRepository(db)

	rows := sqlmock.NewRows([]string{"id", "keyword", "group_id", "status"}).
		AddRow(uint(1), "关键词1", 1, 1).
		AddRow(uint(2), "关键词2", 1, 1)

	mock.ExpectQuery("SELECT (.+) FROM keywords WHERE group_id = (.+) ORDER BY RAND\\(\\) LIMIT (.+)").
		WithArgs(1, 10).
		WillReturnRows(rows)

	keywords, err := repo.RandomByGroupID(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Len(t, keywords, 2)
	assert.Equal(t, "关键词1", keywords[0].Keyword)
}

func TestKeywordRepository_BatchImport(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewKeywordRepository(db)

	keywords := []*models.Keyword{
		{Keyword: "关键词1", GroupID: 1, Status: 1},
		{Keyword: "关键词2", GroupID: 1, Status: 1},
	}

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO keywords")
	mock.ExpectExec("INSERT INTO keywords").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO keywords").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	count, err := repo.BatchImport(context.Background(), keywords)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
