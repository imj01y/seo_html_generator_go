// api/internal/repository/article_repo.go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	models "seo-generator/api/internal/model"
)

type articleRepo struct {
	db *sqlx.DB
}

// NewArticleRepository creates a new article repository
func NewArticleRepository(db *sqlx.DB) ArticleRepository {
	return &articleRepo{db: db}
}

func (r *articleRepo) Create(ctx context.Context, article *models.OriginalArticle) error {
	query := `INSERT INTO original_articles (group_id, title, content, status) VALUES (?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, article.GroupID, article.Title, article.Content, article.Status)
	if err != nil {
		return fmt.Errorf("create article: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	article.ID = uint(id) // 注意:转换 int64 到 uint
	return nil
}

func (r *articleRepo) GetByID(ctx context.Context, id uint) (*models.OriginalArticle, error) {
	query := `
		SELECT id, group_id, source_id, source_url, title, content, status, created_at, updated_at
		FROM original_articles WHERE id = ?
	`

	var article models.OriginalArticle
	err := r.db.GetContext(ctx, &article, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get article by id: %w", err)
	}

	return &article, nil
}

func (r *articleRepo) List(ctx context.Context, groupID int, page, pageSize int) ([]*models.OriginalArticle, int64, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM original_articles WHERE group_id = ?`
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, groupID); err != nil {
		return nil, 0, fmt.Errorf("count articles: %w", err)
	}

	// Query with pagination
	offset := (page - 1) * pageSize
	query := `
		SELECT id, group_id, source_id, source_url, title, content, status, created_at, updated_at
		FROM original_articles
		WHERE group_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`

	var articles []*models.OriginalArticle
	if err := r.db.SelectContext(ctx, &articles, query, groupID, pageSize, offset); err != nil {
		return nil, 0, fmt.Errorf("list articles: %w", err)
	}

	return articles, total, nil
}

func (r *articleRepo) Update(ctx context.Context, article *models.OriginalArticle) error {
	query := `
		UPDATE original_articles SET
			group_id = ?,
			source_id = ?,
			source_url = ?,
			title = ?,
			content = ?,
			status = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		article.GroupID,
		article.SourceID,
		article.SourceURL,
		article.Title,
		article.Content,
		article.Status,
		article.ID,
	)
	if err != nil {
		return fmt.Errorf("update article: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *articleRepo) Delete(ctx context.Context, id uint) error {
	query := `DELETE FROM original_articles WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *articleRepo) BatchImport(ctx context.Context, articles []*models.OriginalArticle) (int64, error) {
	if len(articles) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO original_articles (group_id, source_id, source_url, title, content, status) VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	var count int64
	for _, article := range articles {
		_, err := stmt.ExecContext(ctx, article.GroupID, article.SourceID, article.SourceURL, article.Title, article.Content, article.Status)
		if err != nil {
			// Skip duplicates
			continue
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return count, nil
}

func (r *articleRepo) CountByGroupID(ctx context.Context, groupID int) (int64, error) {
	query := `SELECT COUNT(*) FROM original_articles WHERE group_id = ?`

	var count int64
	if err := r.db.GetContext(ctx, &count, query, groupID); err != nil {
		return 0, fmt.Errorf("count articles by group: %w", err)
	}

	return count, nil
}
