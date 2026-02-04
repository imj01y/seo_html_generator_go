// api/internal/repository/title_repo.go
package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	models "seo-generator/api/internal/model"
)

type titleRepo struct {
	db *sqlx.DB
}

// NewTitleRepository creates a new title repository
func NewTitleRepository(db *sqlx.DB) TitleRepository {
	return &titleRepo{db: db}
}

func (r *titleRepo) BatchCreate(ctx context.Context, titles []*models.Title) error {
	if len(titles) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO titles (group_id, title, batch_id) VALUES (?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	var count int
	for _, title := range titles {
		_, err := stmt.ExecContext(ctx, title.GroupID, title.Title, title.BatchID)
		if err != nil {
			// 仅跳过重复键错误
			if IsDuplicateKeyError(err) {
				continue
			}
			// 其他错误立即返回
			return fmt.Errorf("insert title: %w", err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *titleRepo) RandomByTemplateID(ctx context.Context, templateID int, batchID int64, limit int) ([]*models.Title, error) {
	query := `
		SELECT id, group_id, title, batch_id
		FROM titles
		WHERE group_id = ? AND batch_id = ? AND (used IS NULL OR used = 0)
		ORDER BY RAND()
		LIMIT ?
	`

	var titles []*models.Title
	if err := r.db.SelectContext(ctx, &titles, query, templateID, batchID, limit); err != nil {
		return nil, fmt.Errorf("random titles: %w", err)
	}

	return titles, nil
}

func (r *titleRepo) MarkAsUsed(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE titles SET used = 1 WHERE id IN (?%s)", strings.Repeat(",?", len(ids)-1))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark titles as used: %w", err)
	}

	return nil
}

func (r *titleRepo) CountByTemplateID(ctx context.Context, templateID int) (int64, error) {
	query := `SELECT COUNT(*) FROM titles WHERE group_id = ? AND (used IS NULL OR used = 0)`

	var count int64
	if err := r.db.GetContext(ctx, &count, query, templateID); err != nil {
		return 0, fmt.Errorf("count titles by template: %w", err)
	}

	return count, nil
}

func (r *titleRepo) DeleteByBatchID(ctx context.Context, batchID int64) error {
	query := `DELETE FROM titles WHERE batch_id = ?`

	_, err := r.db.ExecContext(ctx, query, batchID)
	if err != nil {
		return fmt.Errorf("delete titles by batch: %w", err)
	}

	return nil
}
