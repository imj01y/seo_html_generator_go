package api

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// ImagesHandler 图片管理 handler
type ImagesHandler struct {
	db *sqlx.DB
}

// NewImagesHandler 创建 ImagesHandler
func NewImagesHandler(db *sqlx.DB) *ImagesHandler {
	return &ImagesHandler{db: db}
}

// ImageGroup 图片分组
type ImageGroup struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	IsDefault   int       `json:"is_default" db:"is_default"`
	Status      int       `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ImageListItem 图片列表项
type ImageListItem struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	URL       string    `json:"url" db:"url"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ImageGroupCreateRequest 创建分组请求
type ImageGroupCreateRequest struct {
	SiteGroupID int    `json:"site_group_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// ImageGroupUpdateRequest 更新分组请求
type ImageGroupUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *int    `json:"is_default"`
}

// ImageURLUpdateRequest 更新图片URL请求
type ImageURLUpdateRequest struct {
	URL     *string `json:"url"`
	GroupID *int    `json:"group_id"`
	Status  *int    `json:"status"`
}

// ImageBatchIdsRequest 批量ID请求
type ImageBatchIdsRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// ImageBatchStatusRequest 批量状态更新请求
type ImageBatchStatusRequest struct {
	IDs    []int `json:"ids" binding:"required"`
	Status int   `json:"status"`
}

// ImageBatchMoveRequest 批量移动请求
type ImageBatchMoveRequest struct {
	IDs     []int `json:"ids" binding:"required"`
	GroupID int   `json:"group_id" binding:"required"`
}

// ImageDeleteAllRequest 删除全部请求
type ImageDeleteAllRequest struct {
	Confirm bool `json:"confirm" binding:"required"`
	GroupID *int `json:"group_id"`
}

// ImageAddRequest 添加单个图片请求
type ImageAddRequest struct {
	URL     string `json:"url" binding:"required"`
	GroupID int    `json:"group_id"`
}

// ImageBatchAddRequest 批量添加图片请求
type ImageBatchAddRequest struct {
	URLs    []string `json:"urls" binding:"required"`
	GroupID int      `json:"group_id"`
}

// 确保导入包被使用
var (
	_ = sql.ErrNoRows
	_ = fmt.Sprintf
	_ = strconv.Atoi
	_ = strings.TrimSpace
	_ = log.Info
	_ = core.Success
	_ gin.HandlerFunc
)
