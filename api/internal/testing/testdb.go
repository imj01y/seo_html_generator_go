// api/internal/testing/testdb.go
package testing

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// NewMockDB creates a new mock database for testing
func NewMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	cleanup := func() {
		sqlxDB.Close()
	}

	return sqlxDB, mock, cleanup
}

// ExpectBegin expects a transaction to begin
func ExpectBegin(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
}

// ExpectCommit expects a transaction to commit
func ExpectCommit(mock sqlmock.Sqlmock) {
	mock.ExpectCommit()
}

// ExpectRollback expects a transaction to rollback
func ExpectRollback(mock sqlmock.Sqlmock) {
	mock.ExpectRollback()
}
