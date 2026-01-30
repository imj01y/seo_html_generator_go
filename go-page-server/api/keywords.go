// go-page-server/api/keywords.go
package api

import (
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// KeywordsHandler 关键词管理 handler
type KeywordsHandler struct {
	db *sqlx.DB
}

// NewKeywordsHandler 创建 KeywordsHandler
func NewKeywordsHandler(db *sqlx.DB) *KeywordsHandler {
	return &KeywordsHandler{db: db}
}

// KeywordGroup 关键词分组
type KeywordGroup struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	IsDefault   int       `json:"is_default" db:"is_default"`
	Status      int       `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// KeywordListItem 关键词列表项
type KeywordListItem struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	Keyword   string    `json:"keyword" db:"keyword"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// GroupCreateRequest 创建分组请求
type GroupCreateRequest struct {
	SiteGroupID int    `json:"site_group_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// GroupUpdateRequest 更新分组请求
type GroupUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *int    `json:"is_default"`
}

// KeywordUpdateRequest 更新关键词请求
type KeywordUpdateRequest struct {
	Keyword *string `json:"keyword"`
	GroupID *int    `json:"group_id"`
	Status  *int    `json:"status"`
}

// BatchIdsRequest 批量ID请求
type BatchIdsRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// BatchStatusRequest 批量状态更新请求
type BatchStatusRequest struct {
	IDs    []int `json:"ids" binding:"required"`
	Status int   `json:"status"`
}

// BatchMoveRequest 批量移动请求
type BatchMoveRequest struct {
	IDs     []int `json:"ids" binding:"required"`
	GroupID int   `json:"group_id" binding:"required"`
}

// DeleteAllRequest 删除全部请求
type DeleteAllRequest struct {
	Confirm bool `json:"confirm" binding:"required"`
	GroupID *int `json:"group_id"`
}

// KeywordAddRequest 添加单个关键词请求
type KeywordAddRequest struct {
	Keyword string `json:"keyword" binding:"required"`
	GroupID int    `json:"group_id"`
}

// KeywordBatchAddRequest 批量添加关键词请求
type KeywordBatchAddRequest struct {
	Keywords []string `json:"keywords" binding:"required"`
	GroupID  int      `json:"group_id"`
}

// 临时变量来确保导入包被使用（后续实现会用到）
var (
	_ = sql.ErrNoRows
	_ = fmt.Sprintf
	_ = io.EOF
	_ = strconv.Atoi
	_ = strings.TrimSpace
	_ = log.Info
	_ = core.Success
)

// 临时使用 gin.Context 确保导入
var _ *gin.Context
