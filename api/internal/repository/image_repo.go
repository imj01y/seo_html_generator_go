// api/internal/repository/image_repo.go
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

type imageRepo struct {
	db *sqlx.DB
}

// NewImageRepository creates a new image repository
func NewImageRepository(db *sqlx.DB) ImageRepository {
	return &imageRepo{db: db}
}

func (r *imageRepo) Create(ctx context.Context, image *models.Image) error {
	query := `INSERT INTO images (url, group_id, status) VALUES (?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, image.URL, image.GroupID, image.Status)
	if err != nil {
		return fmt.Errorf("create image: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	image.ID = uint(id) // 注意:转换 int64 到 uint
	return nil
}

func (r *imageRepo) GetByID(ctx context.Context, id uint) (*models.Image, error) {
	query := `SELECT id, url, group_id, status FROM images WHERE id = ?`

	var image models.Image
	err := r.db.GetContext(ctx, &image, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get image by id: %w", err)
	}

	return &image, nil
}

func (r *imageRepo) List(ctx context.Context, filter ImageFilter) ([]*models.Image, int64, error) {
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

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM images %s", whereClause)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count images: %w", err)
	}

	// Query with pagination
	query := fmt.Sprintf("SELECT id, url, group_id, status FROM images %s ORDER BY id DESC", whereClause)
	if filter.Pagination != nil {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Pagination.PageSize, filter.Pagination.Offset)
	}

	var images []*models.Image
	if err := r.db.SelectContext(ctx, &images, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list images: %w", err)
	}

	return images, total, nil
}

func (r *imageRepo) RandomByGroupID(ctx context.Context, groupID int, limit int) ([]*models.Image, error) {
	query := `
		SELECT id, url, group_id, status
		FROM images
		WHERE group_id = ? AND status = 1
		ORDER BY RAND()
		LIMIT ?
	`

	var images []*models.Image
	if err := r.db.SelectContext(ctx, &images, query, groupID, limit); err != nil {
		return nil, fmt.Errorf("random images: %w", err)
	}

	return images, nil
}

func (r *imageRepo) BatchImport(ctx context.Context, images []*models.Image) (int64, error) {
	if len(images) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO images (url, group_id, status) VALUES (?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	var count int64
	for _, img := range images {
		_, err := stmt.ExecContext(ctx, img.URL, img.GroupID, img.Status)
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

func (r *imageRepo) Delete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf("DELETE FROM images WHERE id IN (?%s)", strings.Repeat(",?", len(ids)-1))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete images: %w", err)
	}

	return nil
}

func (r *imageRepo) Count(ctx context.Context, filter ImageFilter) (int64, error) {
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

	query := fmt.Sprintf("SELECT COUNT(*) FROM images %s", whereClause)

	var count int64
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count images: %w", err)
	}

	return count, nil
}

func (r *imageRepo) CountByGroupID(ctx context.Context, groupID int) (int64, error) {
	query := `SELECT COUNT(*) FROM images WHERE group_id = ? AND status = 1`

	var count int64
	if err := r.db.GetContext(ctx, &count, query, groupID); err != nil {
		return 0, fmt.Errorf("count images by group: %w", err)
	}

	return count, nil
}
