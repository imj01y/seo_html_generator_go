// api/internal/repository/keyword_repo.go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	models "seo-generator/api/internal/model"
)

type keywordRepo struct {
	db *sqlx.DB
}

// NewKeywordRepository creates a new keyword repository
func NewKeywordRepository(db *sqlx.DB) KeywordRepository {
	return &keywordRepo{db: db}
}

func (r *keywordRepo) Create(ctx context.Context, keyword *models.Keyword) error {
	query := `INSERT INTO keywords (keyword, group_id, status) VALUES (?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, keyword.Keyword, keyword.GroupID, keyword.Status)
	if err != nil {
		return fmt.Errorf("create keyword: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	keyword.ID = uint(id) // 注意:转换 int64 到 uint
	return nil
}

func (r *keywordRepo) GetByID(ctx context.Context, id uint) (*models.Keyword, error) {
	query := `SELECT id, keyword, group_id, status FROM keywords WHERE id = ?`

	var keyword models.Keyword
	err := r.db.GetContext(ctx, &keyword, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get keyword by id: %w", err)
	}

	return &keyword, nil
}

func (r *keywordRepo) List(ctx context.Context, filter KeywordFilter) ([]*models.Keyword, int64, error) {
	whereClauses := []string{}
	args := []interface{}{}

	if filter.GroupID != nil {
		whereClauses = append(whereClauses, "group_id = ?")
		args = append(args, *filter.GroupID)
	}
	if filter.Status != nil {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Keyword != nil && *filter.Keyword != "" {
		whereClauses = append(whereClauses, "keyword LIKE ?")
		args = append(args, "%"+*filter.Keyword+"%")
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM keywords %s", whereClause)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count keywords: %w", err)
	}

	// Query with pagination
	query := fmt.Sprintf("SELECT id, keyword, group_id, status FROM keywords %s ORDER BY id DESC", whereClause)
	if filter.Pagination != nil {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Pagination.PageSize, filter.Pagination.Offset)
	}

	var keywords []*models.Keyword
	if err := r.db.SelectContext(ctx, &keywords, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list keywords: %w", err)
	}

	return keywords, total, nil
}

func (r *keywordRepo) RandomByGroupID(ctx context.Context, groupID int, limit int) ([]*models.Keyword, error) {
	query := `
		SELECT id, keyword, group_id, status
		FROM keywords
		WHERE group_id = ? AND status = 1
		ORDER BY RAND()
		LIMIT ?
	`

	var keywords []*models.Keyword
	if err := r.db.SelectContext(ctx, &keywords, query, groupID, limit); err != nil {
		return nil, fmt.Errorf("random keywords: %w", err)
	}

	return keywords, nil
}

func (r *keywordRepo) BatchImport(ctx context.Context, keywords []*models.Keyword) (int64, error) {
	if len(keywords) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO keywords (keyword, group_id, status) VALUES (?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	var count int64
	for _, kw := range keywords {
		_, err := stmt.ExecContext(ctx, kw.Keyword, kw.GroupID, kw.Status)
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

func (r *keywordRepo) MarkAsUsed(ctx context.Context, id uint) error {
	query := `UPDATE keywords SET status = 0 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark keyword as used: %w", err)
	}
	return nil
}

func (r *keywordRepo) Delete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf("DELETE FROM keywords WHERE id IN (?%s)", strings.Repeat(",?", len(ids)-1))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete keywords: %w", err)
	}

	return nil
}

func (r *keywordRepo) Count(ctx context.Context, filter KeywordFilter) (int64, error) {
	whereClauses := []string{}
	args := []interface{}{}

	if filter.GroupID != nil {
		whereClauses = append(whereClauses, "group_id = ?")
		args = append(args, *filter.GroupID)
	}
	if filter.Status != nil {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, *filter.Status)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM keywords %s", whereClause)

	var count int64
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count keywords: %w", err)
	}

	return count, nil
}

func (r *keywordRepo) CountByGroupID(ctx context.Context, groupID int) (int64, error) {
	query := `SELECT COUNT(*) FROM keywords WHERE group_id = ? AND status = 1`

	var count int64
	if err := r.db.GetContext(ctx, &count, query, groupID); err != nil {
		return 0, fmt.Errorf("count keywords by group: %w", err)
	}

	return count, nil
}
