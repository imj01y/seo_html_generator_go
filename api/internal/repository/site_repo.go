// api/internal/repository/site_repo.go
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

type siteRepo struct {
	db *sqlx.DB
}

// NewSiteRepository creates a new site repository
func NewSiteRepository(db *sqlx.DB) SiteRepository {
	return &siteRepo{db: db}
}

// buildWhereClause 构建 WHERE 子句和参数
func (r *siteRepo) buildWhereClause(filter SiteFilter) (string, []interface{}) {
	whereClauses := []string{}
	args := []interface{}{}

	if filter.SiteGroupID != nil {
		whereClauses = append(whereClauses, "site_group_id = ?")
		args = append(args, *filter.SiteGroupID)
	}
	if filter.Domain != nil && *filter.Domain != "" {
		whereClauses = append(whereClauses, "domain LIKE ?")
		args = append(args, "%"+*filter.Domain+"%")
	}
	if filter.Status != nil {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, *filter.Status)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	return whereClause, args
}

func (r *siteRepo) Create(ctx context.Context, site *models.Site) error {
	query := `INSERT INTO sites (site_group_id, domain, name, template, status) VALUES (?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, site.SiteGroupID, site.Domain, site.Name, site.Template, site.Status)
	if err != nil {
		return fmt.Errorf("create site: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	site.ID = int(id) // 注意:转换 int64 到 int
	return nil
}

func (r *siteRepo) GetByID(ctx context.Context, id int) (*models.Site, error) {
	query := `
		SELECT id, site_group_id, domain, name, template, status,
		       keyword_group_id, image_group_id, article_group_id,
		       icp_number, baidu_token, analytics,
		       created_at, updated_at
		FROM sites WHERE id = ?
	`

	var site models.Site
	err := r.db.GetContext(ctx, &site, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get site by id: %w", err)
	}

	return &site, nil
}

func (r *siteRepo) GetByDomain(ctx context.Context, domain string) (*models.Site, error) {
	query := `
		SELECT id, site_group_id, domain, name, template, status,
		       keyword_group_id, image_group_id, article_group_id,
		       icp_number, baidu_token, analytics,
		       created_at, updated_at
		FROM sites WHERE domain = ?
	`

	var site models.Site
	err := r.db.GetContext(ctx, &site, query, domain)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get site by domain: %w", err)
	}

	return &site, nil
}

func (r *siteRepo) List(ctx context.Context, filter SiteFilter) ([]*models.Site, int64, error) {
	whereClause, args := r.buildWhereClause(filter)

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM sites %s", whereClause)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count sites: %w", err)
	}

	// Query with pagination
	query := fmt.Sprintf(`
		SELECT id, site_group_id, domain, name, template, status,
		       keyword_group_id, image_group_id, article_group_id,
		       icp_number, baidu_token, analytics,
		       created_at, updated_at
		FROM sites %s ORDER BY id DESC
	`, whereClause)

	if filter.Pagination != nil {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Pagination.PageSize, filter.Pagination.Offset)
	}

	var sites []*models.Site
	if err := r.db.SelectContext(ctx, &sites, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list sites: %w", err)
	}

	return sites, total, nil
}

func (r *siteRepo) Update(ctx context.Context, site *models.Site) error {
	query := `
		UPDATE sites SET
			site_group_id = ?,
			domain = ?,
			name = ?,
			template = ?,
			status = ?,
			keyword_group_id = ?,
			image_group_id = ?,
			article_group_id = ?,
			icp_number = ?,
			baidu_token = ?,
			analytics = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		site.SiteGroupID,
		site.Domain,
		site.Name,
		site.Template,
		site.Status,
		site.KeywordGroupID,
		site.ImageGroupID,
		site.ArticleGroupID,
		site.ICPNumber,
		site.BaiduToken,
		site.Analytics,
		site.ID,
	)
	if err != nil {
		return fmt.Errorf("update site: %w", err)
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

func (r *siteRepo) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM sites WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
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

func (r *siteRepo) BatchCreate(ctx context.Context, sites []*models.Site) error {
	if len(sites) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO sites (site_group_id, domain, name, template, status) VALUES (?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	var count int
	for _, site := range sites {
		_, err := stmt.ExecContext(ctx, site.SiteGroupID, site.Domain, site.Name, site.Template, site.Status)
		if err != nil {
			// 仅跳过重复键错误
			if IsDuplicateKeyError(err) {
				continue
			}
			// 其他错误立即返回
			return fmt.Errorf("insert site: %w", err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *siteRepo) Count(ctx context.Context, filter SiteFilter) (int64, error) {
	whereClause, args := r.buildWhereClause(filter)
	query := fmt.Sprintf("SELECT COUNT(*) FROM sites %s", whereClause)

	var count int64
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count sites: %w", err)
	}

	return count, nil
}
