// api/internal/repository/content_repo.go
package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	models "seo-generator/api/internal/model"
)

type contentRepo struct {
	db *sqlx.DB
}

// NewContentRepository creates a new content repository
func NewContentRepository(db *sqlx.DB) ContentRepository {
	return &contentRepo{db: db}
}

func (r *contentRepo) BatchCreate(ctx context.Context, contents []*models.Content) error {
	if len(contents) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO contents (group_id, content, batch_id) VALUES (?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	var count int
	for _, content := range contents {
		_, err := stmt.ExecContext(ctx, content.GroupID, content.Content, content.BatchID)
		if err != nil {
			// 仅跳过重复键错误
			if IsDuplicateKeyError(err) {
				continue
			}
			// 其他错误立即返回
			return fmt.Errorf("insert content: %w", err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *contentRepo) RandomByTemplateID(ctx context.Context, templateID int, batchID int64, limit int) ([]*models.Content, error) {
	query := `
		SELECT id, group_id, content, batch_id
		FROM contents
		WHERE group_id = ? AND batch_id = ? AND status = 1
		ORDER BY RAND()
		LIMIT ?
	`

	var contents []*models.Content
	if err := r.db.SelectContext(ctx, &contents, query, templateID, batchID, limit); err != nil {
		return nil, fmt.Errorf("random contents: %w", err)
	}

	return contents, nil
}

func (r *contentRepo) MarkAsUsed(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE contents SET status = 0 WHERE id IN (?%s)", strings.Repeat(",?", len(ids)-1))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark contents as used: %w", err)
	}

	return nil
}

func (r *contentRepo) CountByTemplateID(ctx context.Context, templateID int) (int64, error) {
	query := `SELECT COUNT(*) FROM contents WHERE group_id = ? AND status = 1`

	var count int64
	if err := r.db.GetContext(ctx, &count, query, templateID); err != nil {
		return 0, fmt.Errorf("count contents by template: %w", err)
	}

	return count, nil
}

func (r *contentRepo) DeleteByBatchID(ctx context.Context, batchID int64) error {
	query := `DELETE FROM contents WHERE batch_id = ?`

	_, err := r.db.ExecContext(ctx, query, batchID)
	if err != nil {
		return fmt.Errorf("delete contents by batch: %w", err)
	}

	return nil
}
