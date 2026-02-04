// api/internal/repository/content_repo_test.go
package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	testutil "seo-generator/api/internal/testing"

	models "seo-generator/api/internal/model"
)

func TestContentRepository_BatchCreate(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewContentRepository(db)

	contents := []*models.Content{
		{GroupID: 1, Content: "内容1", BatchID: 100},
		{GroupID: 1, Content: "内容2", BatchID: 100},
	}

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO contents")
	mock.ExpectExec("INSERT INTO contents").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO contents").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	err := repo.BatchCreate(context.Background(), contents)

	assert.NoError(t, err)
}

func TestContentRepository_RandomByTemplateID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewContentRepository(db)

	rows := sqlmock.NewRows([]string{"id", "group_id", "content", "batch_id"}).
		AddRow(uint64(1), 1, "内容1", 100).
		AddRow(uint64(2), 1, "内容2", 100)

	mock.ExpectQuery("SELECT (.+) FROM contents WHERE group_id = (.+) ORDER BY RAND\\(\\) LIMIT (.+)").
		WithArgs(1, 100, 10).
		WillReturnRows(rows)

	contents, err := repo.RandomByTemplateID(context.Background(), 1, 100, 10)

	assert.NoError(t, err)
	assert.Len(t, contents, 2)
	assert.Equal(t, "内容1", contents[0].Content)
}

func TestContentRepository_MarkAsUsed(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewContentRepository(db)

	ids := []uint64{1, 2, 3}

	mock.ExpectExec("UPDATE contents SET used = 1 WHERE id IN").
		WithArgs(ids[0], ids[1], ids[2]).
		WillReturnResult(sqlmock.NewResult(0, 3))

	err := repo.MarkAsUsed(context.Background(), ids)

	assert.NoError(t, err)
}
