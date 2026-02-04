// api/internal/repository/title_repo_test.go
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

func TestTitleRepository_BatchCreate(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewTitleRepository(db)

	titles := []*models.Title{
		{GroupID: 1, Title: "标题1", BatchID: 100},
		{GroupID: 1, Title: "标题2", BatchID: 100},
	}

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO titles")
	mock.ExpectExec("INSERT INTO titles").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO titles").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	err := repo.BatchCreate(context.Background(), titles)

	assert.NoError(t, err)
}

func TestTitleRepository_RandomByTemplateID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewTitleRepository(db)

	rows := sqlmock.NewRows([]string{"id", "group_id", "title", "batch_id"}).
		AddRow(uint64(1), 1, "标题1", 100).
		AddRow(uint64(2), 1, "标题2", 100)

	mock.ExpectQuery("SELECT (.+) FROM titles WHERE group_id = (.+) ORDER BY RAND\\(\\) LIMIT (.+)").
		WithArgs(1, 100, 10).
		WillReturnRows(rows)

	titles, err := repo.RandomByTemplateID(context.Background(), 1, 100, 10)

	assert.NoError(t, err)
	assert.Len(t, titles, 2)
	assert.Equal(t, "标题1", titles[0].Title)
}

func TestTitleRepository_MarkAsUsed(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewTitleRepository(db)

	ids := []uint64{1, 2, 3}

	mock.ExpectExec("UPDATE titles SET used = 1 WHERE id IN").
		WithArgs(ids[0], ids[1], ids[2]).
		WillReturnResult(sqlmock.NewResult(0, 3))

	err := repo.MarkAsUsed(context.Background(), ids)

	assert.NoError(t, err)
}

func TestTitleRepository_CountByTemplateID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewTitleRepository(db)

	t.Run("success", func(t *testing.T) {
		templateID := 1
		rows := sqlmock.NewRows([]string{"count"}).AddRow(42)

		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM titles WHERE group_id = \\? AND \\(used IS NULL OR used = 0\\)").
			WithArgs(templateID).
			WillReturnRows(rows)

		count, err := repo.CountByTemplateID(context.Background(), templateID)

		assert.NoError(t, err)
		assert.Equal(t, int64(42), count)
	})

	t.Run("error on query", func(t *testing.T) {
		templateID := 1

		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM titles WHERE group_id = \\? AND \\(used IS NULL OR used = 0\\)").
			WithArgs(templateID).
			WillReturnError(sql.ErrConnDone)

		count, err := repo.CountByTemplateID(context.Background(), templateID)

		assert.Error(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestTitleRepository_DeleteByBatchID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewTitleRepository(db)

	t.Run("success", func(t *testing.T) {
		batchID := int64(100)

		mock.ExpectExec("DELETE FROM titles WHERE batch_id = \\?").
			WithArgs(batchID).
			WillReturnResult(sqlmock.NewResult(0, 5))

		err := repo.DeleteByBatchID(context.Background(), batchID)

		assert.NoError(t, err)
	})

	t.Run("error on delete", func(t *testing.T) {
		batchID := int64(100)

		mock.ExpectExec("DELETE FROM titles WHERE batch_id = \\?").
			WithArgs(batchID).
			WillReturnError(sql.ErrConnDone)

		err := repo.DeleteByBatchID(context.Background(), batchID)

		assert.Error(t, err)
	})
}
