// Package repository provides data access layer interfaces and implementations.
// This file defines all repository interfaces for data access operations.
package repository

import (
	"context"
	models "seo-generator/api/internal/model"
)

// SiteRepository 站点数据访问接口
// 提供站点配置的增删改查操作
type SiteRepository interface {
	// Create creates a new site
	Create(ctx context.Context, site *models.Site) error

	// GetByID retrieves a site by its ID
	GetByID(ctx context.Context, id int) (*models.Site, error)

	// GetByDomain retrieves a site by its domain
	GetByDomain(ctx context.Context, domain string) (*models.Site, error)

	// List retrieves sites with filtering and pagination
	// Returns sites slice, total count, and error
	List(ctx context.Context, filter SiteFilter) ([]*models.Site, int64, error)

	// Update updates an existing site
	Update(ctx context.Context, site *models.Site) error

	// Delete deletes a site by ID
	Delete(ctx context.Context, id int) error

	// BatchCreate creates multiple sites in a single transaction
	BatchCreate(ctx context.Context, sites []*models.Site) error

	// Count returns the total count of sites matching the filter
	Count(ctx context.Context, filter SiteFilter) (int64, error)
}

// KeywordRepository 关键词数据访问接口
// 提供关键词的增删改查、随机获取、批量导入等操作
type KeywordRepository interface {
	// Create creates a new keyword
	Create(ctx context.Context, keyword *models.Keyword) error

	// GetByID retrieves a keyword by its ID
	GetByID(ctx context.Context, id uint) (*models.Keyword, error)

	// List retrieves keywords with filtering and pagination
	// Returns keywords slice, total count, and error
	List(ctx context.Context, filter KeywordFilter) ([]*models.Keyword, int64, error)

	// RandomByGroupID retrieves random keywords from a specific group
	// Used for template rendering
	RandomByGroupID(ctx context.Context, groupID int, limit int) ([]*models.Keyword, error)

	// BatchImport imports multiple keywords in a single transaction
	// Returns the number of successfully imported keywords and error
	BatchImport(ctx context.Context, keywords []*models.Keyword) (int64, error)

	// MarkAsUsed marks a keyword as used (status = 0)
	MarkAsUsed(ctx context.Context, id uint) error

	// Delete deletes keywords by IDs
	Delete(ctx context.Context, ids []uint) error

	// Count returns the total count of keywords matching the filter
	Count(ctx context.Context, filter KeywordFilter) (int64, error)

	// CountByGroupID returns the count of keywords in a specific group
	CountByGroupID(ctx context.Context, groupID int) (int64, error)
}

// ImageRepository 图片数据访问接口
// 提供图片的增删改查、随机获取、批量导入等操作
type ImageRepository interface {
	// Create creates a new image
	Create(ctx context.Context, image *models.Image) error

	// GetByID retrieves an image by its ID
	GetByID(ctx context.Context, id uint) (*models.Image, error)

	// List retrieves images with filtering and pagination
	// Returns images slice, total count, and error
	List(ctx context.Context, filter ImageFilter) ([]*models.Image, int64, error)

	// RandomByGroupID retrieves random images from a specific group
	// Used for template rendering
	RandomByGroupID(ctx context.Context, groupID int, limit int) ([]*models.Image, error)

	// BatchImport imports multiple images in a single transaction
	// Returns the number of successfully imported images and error
	BatchImport(ctx context.Context, images []*models.Image) (int64, error)

	// Delete deletes images by IDs
	Delete(ctx context.Context, ids []uint) error

	// Count returns the total count of images matching the filter
	Count(ctx context.Context, filter ImageFilter) (int64, error)

	// CountByGroupID returns the count of images in a specific group
	CountByGroupID(ctx context.Context, groupID int) (int64, error)
}

// ArticleRepository 文章数据访问接口
// 提供原始文章的增删改查、批量导入等操作
type ArticleRepository interface {
	// Create creates a new article
	Create(ctx context.Context, article *models.OriginalArticle) error

	// GetByID retrieves an article by its ID
	GetByID(ctx context.Context, id uint) (*models.OriginalArticle, error)

	// List retrieves articles with pagination
	// Returns articles slice, total count, and error
	List(ctx context.Context, groupID int, page, pageSize int) ([]*models.OriginalArticle, int64, error)

	// Update updates an existing article
	Update(ctx context.Context, article *models.OriginalArticle) error

	// Delete deletes an article by ID
	Delete(ctx context.Context, id uint) error

	// BatchImport imports multiple articles in a single transaction
	// Returns the number of successfully imported articles and error
	BatchImport(ctx context.Context, articles []*models.OriginalArticle) (int64, error)

	// CountByGroupID returns the count of articles in a specific group
	CountByGroupID(ctx context.Context, groupID int) (int64, error)
}

// TitleRepository 标题数据访问接口
// 提供标题的批量创建、随机获取、标记使用等操作
type TitleRepository interface {
	// BatchCreate creates multiple titles in a single transaction
	BatchCreate(ctx context.Context, titles []*models.Title) error

	// RandomByTemplateID retrieves random unused titles for a template
	// batchID is used to identify the generation batch
	RandomByTemplateID(ctx context.Context, templateID int, batchID int64, limit int) ([]*models.Title, error)

	// MarkAsUsed marks titles as used by IDs
	MarkAsUsed(ctx context.Context, ids []uint64) error

	// CountByTemplateID returns the count of titles for a specific template
	CountByTemplateID(ctx context.Context, templateID int) (int64, error)

	// DeleteByBatchID deletes all titles in a specific batch
	DeleteByBatchID(ctx context.Context, batchID int64) error
}

// ContentRepository 正文数据访问接口
// 提供正文的批量创建、随机获取、标记使用等操作
type ContentRepository interface {
	// BatchCreate creates multiple contents in a single transaction
	BatchCreate(ctx context.Context, contents []*models.Content) error

	// RandomByTemplateID retrieves random unused contents for a template
	// batchID is used to identify the generation batch
	RandomByTemplateID(ctx context.Context, templateID int, batchID int64, limit int) ([]*models.Content, error)

	// MarkAsUsed marks contents as used by IDs
	MarkAsUsed(ctx context.Context, ids []uint64) error

	// CountByTemplateID returns the count of contents for a specific template
	CountByTemplateID(ctx context.Context, templateID int) (int64, error)

	// DeleteByBatchID deletes all contents in a specific batch
	DeleteByBatchID(ctx context.Context, batchID int64) error
}
