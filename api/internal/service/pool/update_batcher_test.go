package pool

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	testutil "seo-generator/api/internal/testing"
)

func TestUpdateBatcher(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	config := BatcherConfig{
		MaxBatch:      10,
		FlushInterval: 100 * time.Millisecond,
	}

	batcher := NewUpdateBatcher(db, config)
	defer batcher.Stop()

	t.Run("batch updates on interval", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE contents SET status = 0 WHERE id IN").
			WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectCommit()

		batcher.Add(UpdateTask{Table: "contents", ID: 1})
		batcher.Add(UpdateTask{Table: "contents", ID: 2})

		time.Sleep(150 * time.Millisecond) // 等待自动刷新

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpdateBatcherMaxBatch(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	config := BatcherConfig{
		MaxBatch:      3,
		FlushInterval: 10 * time.Second, // 长间隔，确保不会被定时器触发
	}

	batcher := NewUpdateBatcher(db, config)
	defer batcher.Stop()

	t.Run("immediate flush on max batch", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE contents SET status = 0 WHERE id IN").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()

		// 添加 3 个任务（达到 MaxBatch）
		batcher.Add(UpdateTask{Table: "contents", ID: 1})
		batcher.Add(UpdateTask{Table: "contents", ID: 2})
		batcher.Add(UpdateTask{Table: "contents", ID: 3})

		time.Sleep(50 * time.Millisecond) // 短暂等待处理

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpdateBatcherMultipleTables(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	config := BatcherConfig{
		MaxBatch:      10,
		FlushInterval: 100 * time.Millisecond,
	}

	batcher := NewUpdateBatcher(db, config)
	defer batcher.Stop()

	t.Run("batch updates for multiple tables", func(t *testing.T) {
		// Map iteration order is not deterministic, so allow any order
		mock.MatchExpectationsInOrder(false)

		mock.ExpectBegin()
		// 两个表的更新可能以任意顺序执行
		mock.ExpectExec("UPDATE contents SET status = 0 WHERE id IN").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE titles SET status = 0 WHERE id IN").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		batcher.Add(UpdateTask{Table: "contents", ID: 1})
		batcher.Add(UpdateTask{Table: "titles", ID: 2})

		time.Sleep(150 * time.Millisecond)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpdateBatcherInvalidTable(t *testing.T) {
	db, _, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	config := BatcherConfig{
		MaxBatch:      10,
		FlushInterval: 100 * time.Millisecond,
	}

	batcher := NewUpdateBatcher(db, config)
	defer batcher.Stop()

	t.Run("reject invalid table names", func(t *testing.T) {
		// 添加无效表名的任务（应该被跳过，不会执行 SQL）
		batcher.Add(UpdateTask{Table: "invalid_table", ID: 1})
		batcher.Add(UpdateTask{Table: "; DROP TABLE users;", ID: 2})

		time.Sleep(150 * time.Millisecond)

		// 没有任何 SQL 应该被执行
	})
}

func TestUpdateBatcherStop(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	config := BatcherConfig{
		MaxBatch:      100, // 大批量，确保不会自动触发
		FlushInterval: 10 * time.Second,
	}

	batcher := NewUpdateBatcher(db, config)

	t.Run("flush pending on stop", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE contents SET status = 0 WHERE id IN").
			WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectCommit()

		batcher.Add(UpdateTask{Table: "contents", ID: 1})
		batcher.Add(UpdateTask{Table: "contents", ID: 2})

		// Stop 应该刷新所有待处理任务
		batcher.Stop()

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
