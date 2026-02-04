// Package repository provides data access layer interfaces and implementations.
// This file defines filter and pagination structures for repository queries.
package repository

// Pagination 分页参数
type Pagination struct {
	Page     int
	PageSize int
	Offset   int
}

// NewPagination creates a new pagination with validation
// Page starts from 1, default PageSize is 10, max PageSize is 100
func NewPagination(page, pageSize int) *Pagination {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return &Pagination{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}
}

// SiteFilter 站点查询过滤器
type SiteFilter struct {
	SiteGroupID *int
	Domain      *string
	Status      *int
	Pagination  *Pagination
}

// KeywordFilter 关键词查询过滤器
type KeywordFilter struct {
	GroupID    *int
	Status     *int
	Keyword    *string
	Pagination *Pagination
}

// ImageFilter 图片查询过滤器
type ImageFilter struct {
	GroupID    *int
	Status     *int
	Pagination *Pagination
}
