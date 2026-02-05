# Go API 后端安全重构实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 安全地重构 Go API 后端代码，改进代码结构、消除重复、统一错误处理、优化性能，同时确保系统稳定性和向后兼容。

**Architecture:** 采用渐进式重构策略，每个阶段独立验证。建立完整的 Repository 层抽象数据访问，拆分大文件到合理粒度，使用依赖注入替代全局单例，提取通用 CRUD 逻辑消除重复代码。所有变更保持向后兼容，通过测试覆盖和双写验证确保安全性。

**Tech Stack:** Go 1.24+, sqlx, zerolog, Gin, testify/assert

**重构原则:**
- **安全第一**: 每个阶段独立测试和验证
- **渐进式**: 保持向后兼容，新旧代码并存
- **可回滚**: 每个阶段打 Git tag，便于回滚
- **测试覆盖**: TDD 驱动，先写测试再重构
- **文档同步**: 更新相关文档和注释

---

## 阶段 0: 准备工作和测试框架建立

### Task 0.1: 创建测试基础设施

**目标**: 建立测试框架，提供 mock 数据库和测试工具

**Files:**
- Create: `api/internal/testing/testdb.go`
- Create: `api/internal/testing/fixtures.go`
- Create: `api/internal/testing/mocks.go`

**Step 1: 创建测试数据库辅助工具**

```go
// api/internal/testing/testdb.go
package testing

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

// NewMockDB creates a new mock database for testing
func NewMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	cleanup := func() {
		sqlxDB.Close()
	}

	return sqlxDB, mock, cleanup
}

// ExpectBegin expects a transaction to begin
func ExpectBegin(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
}

// ExpectCommit expects a transaction to commit
func ExpectCommit(mock sqlmock.Sqlmock) {
	mock.ExpectCommit()
}

// ExpectRollback expects a transaction to rollback
func ExpectRollback(mock sqlmock.Sqlmock) {
	mock.ExpectRollback()
}
```

**Step 2: 创建测试数据固件**

```go
// api/internal/testing/fixtures.go
package testing

import "time"

// Fixtures 提供测试数据
type Fixtures struct{}

// NewFixtures creates a new fixtures instance
func NewFixtures() *Fixtures {
	return &Fixtures{}
}

// ValidSite returns a valid site for testing
func (f *Fixtures) ValidSite() map[string]interface{} {
	return map[string]interface{}{
		"id":               1,
		"site_group_id":    1,
		"domain":           "example.com",
		"template_id":      1,
		"keyword_group_id": 1,
		"image_group_id":   1,
		"status":           1,
		"created_at":       time.Now(),
		"updated_at":       time.Now(),
	}
}

// ValidKeyword returns a valid keyword for testing
func (f *Fixtures) ValidKeyword() map[string]interface{} {
	return map[string]interface{}{
		"id":       int64(1),
		"keyword":  "测试关键词",
		"group_id": 1,
		"status":   1,
	}
}

// ValidImage returns a valid image for testing
func (f *Fixtures) ValidImage() map[string]interface{} {
	return map[string]interface{}{
		"id":       int64(1),
		"url":      "https://example.com/image.jpg",
		"group_id": 1,
		"status":   1,
	}
}
```

**Step 3: 安装测试依赖**

Run:
```bash
cd api
go get github.com/DATA-DOG/go-sqlmock
go get github.com/stretchr/testify
go mod tidy
```

Expected: Dependencies installed successfully

**Step 4: 验证测试工具**

创建测试文件验证工具可用:

```go
// api/internal/testing/testdb_test.go
package testing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMockDB(t *testing.T) {
	db, mock, cleanup := NewMockDB(t)
	defer cleanup()

	assert.NotNil(t, db)
	assert.NotNil(t, mock)
}

func TestFixtures(t *testing.T) {
	fixtures := NewFixtures()

	site := fixtures.ValidSite()
	assert.Equal(t, 1, site["id"])
	assert.Equal(t, "example.com", site["domain"])

	keyword := fixtures.ValidKeyword()
	assert.Equal(t, int64(1), keyword["id"])

	image := fixtures.ValidImage()
	assert.Equal(t, int64(1), image["id"])
}
```

Run: `cd api && go test ./internal/testing/... -v`

Expected: PASS

**Step 5: Commit 测试基础设施**

```bash
git add api/internal/testing/
git commit -m "test: add testing infrastructure with mock DB and fixtures"
```

---

## 阶段 1: 建立 Repository 层

### Task 1.1: 定义 Repository 接口

**目标**: 创建所有数据访问的接口定义，为后续实现提供契约

**Files:**
- Create: `api/internal/repository/interfaces.go`
- Create: `api/internal/repository/errors.go`
- Create: `api/internal/repository/filters.go`

**Step 1: 定义通用错误类型**

```go
// api/internal/repository/errors.go
package repository

import "errors"

var (
	// ErrNotFound is returned when a record is not found
	ErrNotFound = errors.New("record not found")

	// ErrDuplicateEntry is returned when a duplicate entry is detected
	ErrDuplicateEntry = errors.New("duplicate entry")

	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")

	// ErrTransactionFailed is returned when a transaction fails
	ErrTransactionFailed = errors.New("transaction failed")
)
```

**Step 2: 定义过滤器和分页结构**

```go
// api/internal/repository/filters.go
package repository

// Pagination 分页参数
type Pagination struct {
	Page     int
	PageSize int
	Offset   int
}

// NewPagination creates a new pagination
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
```

**Step 3: 定义 Repository 接口**

```go
// api/internal/repository/interfaces.go
package repository

import (
	"context"
	"seo-generator/api/internal/model"
)

// SiteRepository 站点数据访问接口
type SiteRepository interface {
	Create(ctx context.Context, site *model.Site) error
	GetByID(ctx context.Context, id int) (*model.Site, error)
	GetByDomain(ctx context.Context, domain string) (*model.Site, error)
	List(ctx context.Context, filter SiteFilter) ([]*model.Site, int64, error)
	Update(ctx context.Context, site *model.Site) error
	Delete(ctx context.Context, id int) error
	BatchCreate(ctx context.Context, sites []*model.Site) error
	Count(ctx context.Context, filter SiteFilter) (int64, error)
}

// KeywordRepository 关键词数据访问接口
type KeywordRepository interface {
	Create(ctx context.Context, keyword *model.Keyword) error
	GetByID(ctx context.Context, id int64) (*model.Keyword, error)
	List(ctx context.Context, filter KeywordFilter) ([]*model.Keyword, int64, error)
	RandomByGroupID(ctx context.Context, groupID int, limit int) ([]*model.Keyword, error)
	BatchImport(ctx context.Context, keywords []*model.Keyword) (int64, error)
	MarkAsUsed(ctx context.Context, id int64) error
	Delete(ctx context.Context, ids []int64) error
	Count(ctx context.Context, filter KeywordFilter) (int64, error)
	CountByGroupID(ctx context.Context, groupID int) (int64, error)
}

// ImageRepository 图片数据访问接口
type ImageRepository interface {
	Create(ctx context.Context, image *model.Image) error
	GetByID(ctx context.Context, id int64) (*model.Image, error)
	List(ctx context.Context, filter ImageFilter) ([]*model.Image, int64, error)
	RandomByGroupID(ctx context.Context, groupID int, limit int) ([]*model.Image, error)
	BatchImport(ctx context.Context, images []*model.Image) (int64, error)
	Delete(ctx context.Context, ids []int64) error
	Count(ctx context.Context, filter ImageFilter) (int64, error)
	CountByGroupID(ctx context.Context, groupID int) (int64, error)
}

// ArticleRepository 文章数据访问接口
type ArticleRepository interface {
	Create(ctx context.Context, article *model.OriginalArticle) error
	GetByID(ctx context.Context, id int) (*model.OriginalArticle, error)
	List(ctx context.Context, groupID int, page, pageSize int) ([]*model.OriginalArticle, int64, error)
	Update(ctx context.Context, article *model.OriginalArticle) error
	Delete(ctx context.Context, id int) error
	BatchImport(ctx context.Context, articles []*model.OriginalArticle) (int64, error)
	CountByGroupID(ctx context.Context, groupID int) (int64, error)
}

// TitleRepository 标题数据访问接口
type TitleRepository interface {
	BatchCreate(ctx context.Context, titles []*model.Title) error
	RandomByTemplateID(ctx context.Context, templateID int, batchID int64, limit int) ([]*model.Title, error)
	MarkAsUsed(ctx context.Context, ids []int64) error
	CountByTemplateID(ctx context.Context, templateID int) (int64, error)
	DeleteByBatchID(ctx context.Context, batchID int64) error
}

// ContentRepository 正文数据访问接口
type ContentRepository interface {
	BatchCreate(ctx context.Context, contents []*model.Content) error
	RandomByTemplateID(ctx context.Context, templateID int, batchID int64, limit int) ([]*model.Content, error)
	MarkAsUsed(ctx context.Context, ids []int64) error
	CountByTemplateID(ctx context.Context, templateID int) (int64, error)
	DeleteByBatchID(ctx context.Context, batchID int64) error
}
```

**Step 4: 验证接口编译**

Run: `cd api && go build ./internal/repository/...`

Expected: 编译成功（即使没有实现）

**Step 5: Commit 接口定义**

```bash
git add api/internal/repository/interfaces.go
git add api/internal/repository/errors.go
git add api/internal/repository/filters.go
git commit -m "refactor(repo): define repository interfaces and error types"
```

---

### Task 1.2: 实现 KeywordRepository（示例实现）

**目标**: 实现关键词 Repository 作为模板，后续其他 Repository 可参考

**Files:**
- Create: `api/internal/repository/keyword_repo.go`
- Create: `api/internal/repository/keyword_repo_test.go`

**Step 1: 编写 KeywordRepository 测试（失败的测试）**

```go
// api/internal/repository/keyword_repo_test.go
package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"seo-generator/api/internal/model"
	testutil "seo-generator/api/internal/testing"
)

func TestKeywordRepository_GetByID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewKeywordRepository(db)

	t.Run("success", func(t *testing.T) {
		expectedID := int64(1)
		rows := sqlmock.NewRows([]string{"id", "keyword", "group_id", "status"}).
			AddRow(expectedID, "测试关键词", 1, 1)

		mock.ExpectQuery("SELECT (.+) FROM keywords WHERE id = ?").
			WithArgs(expectedID).
			WillReturnRows(rows)

		keyword, err := repo.GetByID(context.Background(), expectedID)

		assert.NoError(t, err)
		assert.NotNil(t, keyword)
		assert.Equal(t, expectedID, keyword.ID)
		assert.Equal(t, "测试关键词", keyword.Keyword)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM keywords WHERE id = ?").
			WithArgs(int64(999)).
			WillReturnError(errors.New("sql: no rows in result set"))

		keyword, err := repo.GetByID(context.Background(), 999)

		assert.Error(t, err)
		assert.Nil(t, keyword)
		assert.Equal(t, ErrNotFound, err)
	})
}

func TestKeywordRepository_RandomByGroupID(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewKeywordRepository(db)

	rows := sqlmock.NewRows([]string{"id", "keyword", "group_id", "status"}).
		AddRow(int64(1), "关键词1", 1, 1).
		AddRow(int64(2), "关键词2", 1, 1)

	mock.ExpectQuery("SELECT (.+) FROM keywords WHERE group_id = (.+) ORDER BY RAND\\(\\) LIMIT (.+)").
		WithArgs(1, 10).
		WillReturnRows(rows)

	keywords, err := repo.RandomByGroupID(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.Len(t, keywords, 2)
	assert.Equal(t, "关键词1", keywords[0].Keyword)
}

func TestKeywordRepository_BatchImport(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	repo := NewKeywordRepository(db)

	keywords := []*model.Keyword{
		{Keyword: "关键词1", GroupID: 1, Status: 1},
		{Keyword: "关键词2", GroupID: 1, Status: 1},
	}

	testutil.ExpectBegin(mock)
	mock.ExpectExec("INSERT INTO keywords").
		WillReturnResult(sqlmock.NewResult(1, 2))
	testutil.ExpectCommit(mock)

	count, err := repo.BatchImport(context.Background(), keywords)

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
```

**Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/repository -v -run TestKeywordRepository`

Expected: FAIL - 函数未定义

**Step 3: 实现 KeywordRepository**

```go
// api/internal/repository/keyword_repo.go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"seo-generator/api/internal/model"
)

type keywordRepo struct {
	db *sqlx.DB
}

// NewKeywordRepository creates a new keyword repository
func NewKeywordRepository(db *sqlx.DB) KeywordRepository {
	return &keywordRepo{db: db}
}

func (r *keywordRepo) Create(ctx context.Context, keyword *model.Keyword) error {
	query := `INSERT INTO keywords (keyword, group_id, status) VALUES (?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, keyword.Keyword, keyword.GroupID, keyword.Status)
	if err != nil {
		return fmt.Errorf("create keyword: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	keyword.ID = id
	return nil
}

func (r *keywordRepo) GetByID(ctx context.Context, id int64) (*model.Keyword, error) {
	query := `SELECT id, keyword, group_id, status FROM keywords WHERE id = ?`

	var keyword model.Keyword
	err := r.db.GetContext(ctx, &keyword, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get keyword by id: %w", err)
	}

	return &keyword, nil
}

func (r *keywordRepo) List(ctx context.Context, filter KeywordFilter) ([]*model.Keyword, int64, error) {
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

	var keywords []*model.Keyword
	if err := r.db.SelectContext(ctx, &keywords, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list keywords: %w", err)
	}

	return keywords, total, nil
}

func (r *keywordRepo) RandomByGroupID(ctx context.Context, groupID int, limit int) ([]*model.Keyword, error) {
	query := `
		SELECT id, keyword, group_id, status
		FROM keywords
		WHERE group_id = ? AND status = 1
		ORDER BY RAND()
		LIMIT ?
	`

	var keywords []*model.Keyword
	if err := r.db.SelectContext(ctx, &keywords, query, groupID, limit); err != nil {
		return nil, fmt.Errorf("random keywords: %w", err)
	}

	return keywords, nil
}

func (r *keywordRepo) BatchImport(ctx context.Context, keywords []*model.Keyword) (int64, error) {
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

func (r *keywordRepo) MarkAsUsed(ctx context.Context, id int64) error {
	query := `UPDATE keywords SET status = 0 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark keyword as used: %w", err)
	}
	return nil
}

func (r *keywordRepo) Delete(ctx context.Context, ids []int64) error {
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
```

**Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/repository -v -run TestKeywordRepository`

Expected: PASS

**Step 5: Commit KeywordRepository**

```bash
git add api/internal/repository/keyword_repo.go
git add api/internal/repository/keyword_repo_test.go
git commit -m "feat(repo): implement KeywordRepository with tests"
```

---

### Task 1.3: 实现其他 Repository（批量完成）

**目标**: 按照 KeywordRepository 的模式实现其他 Repository

**Files:**
- Create: `api/internal/repository/image_repo.go`
- Create: `api/internal/repository/image_repo_test.go`
- Create: `api/internal/repository/site_repo.go`
- Create: `api/internal/repository/site_repo_test.go`
- Create: `api/internal/repository/article_repo.go`
- Create: `api/internal/repository/article_repo_test.go`
- Create: `api/internal/repository/title_repo.go`
- Create: `api/internal/repository/title_repo_test.go`
- Create: `api/internal/repository/content_repo.go`
- Create: `api/internal/repository/content_repo_test.go`

**注意**: 由于 ImageRepository 和 KeywordRepository 结构类似，可以复用大部分代码逻辑。其他 Repository 依此类推。

**Step 1: 实现 ImageRepository（参考 KeywordRepository）**

参考 `keyword_repo.go` 的结构，替换表名和字段名即可。

**Step 2: 实现 SiteRepository**

```go
// api/internal/repository/site_repo.go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"seo-generator/api/internal/model"
)

type siteRepo struct {
	db *sqlx.DB
}

func NewSiteRepository(db *sqlx.DB) SiteRepository {
	return &siteRepo{db: db}
}

func (r *siteRepo) Create(ctx context.Context, site *model.Site) error {
	query := `
		INSERT INTO sites (site_group_id, domain, template_id, keyword_group_id, image_group_id, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		site.SiteGroupID, site.Domain, site.TemplateID,
		site.KeywordGroupID, site.ImageGroupID, site.Status)
	if err != nil {
		return fmt.Errorf("create site: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	site.ID = int(id)
	return nil
}

func (r *siteRepo) GetByID(ctx context.Context, id int) (*model.Site, error) {
	query := `
		SELECT id, site_group_id, domain, template_id, keyword_group_id,
		       image_group_id, status, created_at, updated_at
		FROM sites WHERE id = ?
	`

	var site model.Site
	err := r.db.GetContext(ctx, &site, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get site by id: %w", err)
	}

	return &site, nil
}

func (r *siteRepo) GetByDomain(ctx context.Context, domain string) (*model.Site, error) {
	query := `
		SELECT id, site_group_id, domain, template_id, keyword_group_id,
		       image_group_id, status, created_at, updated_at
		FROM sites WHERE domain = ?
	`

	var site model.Site
	err := r.db.GetContext(ctx, &site, query, domain)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get site by domain: %w", err)
	}

	return &site, nil
}

func (r *siteRepo) List(ctx context.Context, filter SiteFilter) ([]*model.Site, int64, error) {
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

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM sites %s", whereClause)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count sites: %w", err)
	}

	// Query with pagination
	query := fmt.Sprintf(`
		SELECT id, site_group_id, domain, template_id, keyword_group_id,
		       image_group_id, status, created_at, updated_at
		FROM sites %s ORDER BY id DESC
	`, whereClause)

	if filter.Pagination != nil {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Pagination.PageSize, filter.Pagination.Offset)
	}

	var sites []*model.Site
	if err := r.db.SelectContext(ctx, &sites, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list sites: %w", err)
	}

	return sites, total, nil
}

func (r *siteRepo) Update(ctx context.Context, site *model.Site) error {
	query := `
		UPDATE sites
		SET site_group_id = ?, domain = ?, template_id = ?,
		    keyword_group_id = ?, image_group_id = ?, status = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		site.SiteGroupID, site.Domain, site.TemplateID,
		site.KeywordGroupID, site.ImageGroupID, site.Status, site.ID)
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

func (r *siteRepo) BatchCreate(ctx context.Context, sites []*model.Site) error {
	if len(sites) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO sites (site_group_id, domain, template_id, keyword_group_id, image_group_id, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, site := range sites {
		_, err := stmt.ExecContext(ctx, site.SiteGroupID, site.Domain, site.TemplateID,
			site.KeywordGroupID, site.ImageGroupID, site.Status)
		if err != nil {
			return fmt.Errorf("insert site %s: %w", site.Domain, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *siteRepo) Count(ctx context.Context, filter SiteFilter) (int64, error) {
	whereClauses := []string{}
	args := []interface{}{}

	if filter.SiteGroupID != nil {
		whereClauses = append(whereClauses, "site_group_id = ?")
		args = append(args, *filter.SiteGroupID)
	}
	if filter.Status != nil {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, *filter.Status)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM sites %s", whereClause)

	var count int64
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count sites: %w", err)
	}

	return count, nil
}
```

**Step 3: 为每个 Repository 编写测试**

参考 `keyword_repo_test.go` 的模式编写测试。

**Step 4: 运行所有 Repository 测试**

Run: `cd api && go test ./internal/repository/... -v`

Expected: 所有测试通过

**Step 5: Commit 所有 Repository**

```bash
git add api/internal/repository/
git commit -m "feat(repo): implement all repository interfaces with tests"
```

---

## 阶段 2: 统一错误处理

### Task 2.1: 定义应用错误类型

**目标**: 创建统一的错误处理机制，替代当前混乱的错误响应

**Files:**
- Create: `api/internal/service/apperror.go`
- Create: `api/internal/service/apperror_test.go`

**Step 1: 编写错误类型测试**

```go
// api/internal/service/apperror_test.go
package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError(t *testing.T) {
	t.Run("database error", func(t *testing.T) {
		dbErr := errors.New("connection failed")
		appErr := NewDatabaseError(dbErr)

		assert.Equal(t, ErrDatabase, appErr.Code)
		assert.Contains(t, appErr.Error(), "Database operation failed")
		assert.NotNil(t, appErr.Err)
	})

	t.Run("validation error", func(t *testing.T) {
		appErr := NewValidationError("email", "invalid format")

		assert.Equal(t, ErrValidation, appErr.Code)
		assert.Contains(t, appErr.Error(), "Validation failed")
		assert.NotNil(t, appErr.Details)
	})

	t.Run("not found error", func(t *testing.T) {
		appErr := NewNotFoundError("Site", 123)

		assert.Equal(t, ErrNotFound, appErr.Code)
		assert.Contains(t, appErr.Error(), "Site")
		assert.Contains(t, appErr.Error(), "123")
	})
}

func TestErrorChaining(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := NewDatabaseError(originalErr)

	assert.True(t, errors.Is(appErr, originalErr))
}
```

**Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/service -v -run TestAppError`

Expected: FAIL

**Step 3: 实现 AppError**

```go
// api/internal/service/apperror.go
package core

import (
	"errors"
	"fmt"
)

// AppError 应用错误类型
type AppError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Err     error       `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// 预定义错误构造器

// NewDatabaseError creates a database error
func NewDatabaseError(err error) *AppError {
	return &AppError{
		Code:    ErrDatabase,
		Message: "Database operation failed",
		Err:     err,
	}
}

// NewValidationError creates a validation error
func NewValidationError(field string, reason string) *AppError {
	return &AppError{
		Code:    ErrValidation,
		Message: "Validation failed",
		Details: map[string]string{field: reason},
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string, id interface{}) *AppError {
	return &AppError{
		Code:    ErrNotFound,
		Message: fmt.Sprintf("%s not found: %v", resource, id),
	}
}

// NewCachePoolEmptyError creates a cache pool empty error
func NewCachePoolEmptyError(poolType string, groupID int) *AppError {
	return &AppError{
		Code:    ErrCachePoolEmpty,
		Message: fmt.Sprintf("Cache pool empty: %s (group %d)", poolType, groupID),
		Details: map[string]interface{}{
			"pool_type": poolType,
			"group_id":  groupID,
		},
	}
}

// NewInternalError creates an internal server error
func NewInternalError(err error) *AppError {
	return &AppError{
		Code:    ErrInternal,
		Message: "Internal server error",
		Err:     err,
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError extracts AppError from error
func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}
```

**Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/service -v -run TestAppError`

Expected: PASS

**Step 5: Commit 错误类型**

```bash
git add api/internal/service/apperror.go
git add api/internal/service/apperror_test.go
git commit -m "feat(error): add unified AppError type with constructors"
```

---

### Task 2.2: 创建错误处理中间件

**目标**: 统一 HTTP 错误响应格式

**Files:**
- Create: `api/internal/handler/error_middleware.go`
- Create: `api/internal/handler/error_middleware_test.go`

**Step 1: 编写中间件测试**

```go
// api/internal/handler/error_middleware_test.go
package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"seo-generator/api/internal/service"
)

func TestErrorMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("handles AppError", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Error(core.NewValidationError("email", "invalid"))

		ErrorHandlerMiddleware()(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), core.ErrValidation)
	})

	t.Run("handles generic error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Error(errors.New("generic error"))

		ErrorHandlerMiddleware()(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("no error passes through", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		ErrorHandlerMiddleware()(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
```

**Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/handler -v -run TestErrorMiddleware`

Expected: FAIL

**Step 3: 实现错误中间件**

```go
// api/internal/handler/error_middleware.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/service"
)

// ErrorHandlerMiddleware 统一错误处理中间件
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err

		// 尝试提取 AppError
		appErr := core.GetAppError(err)
		if appErr != nil {
			handleAppError(c, appErr)
			return
		}

		// 处理未知错误
		log.Error().Err(err).Msg("Unhandled error")
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "Internal server error",
		})
	}
}

func handleAppError(c *gin.Context, appErr *core.AppError) {
	statusCode := getStatusCodeFromErrorCode(appErr.Code)

	log.Error().
		Str("code", appErr.Code).
		Err(appErr.Err).
		Interface("details", appErr.Details).
		Msg(appErr.Message)

	response := gin.H{
		"code":    appErr.Code,
		"message": appErr.Message,
	}

	if appErr.Details != nil {
		response["details"] = appErr.Details
	}

	c.JSON(statusCode, response)
}

func getStatusCodeFromErrorCode(code string) int {
	switch code {
	case core.ErrValidation:
		return http.StatusBadRequest
	case core.ErrNotFound:
		return http.StatusNotFound
	case core.ErrDatabase:
		return http.StatusInternalServerError
	case core.ErrCachePoolEmpty:
		return http.StatusServiceUnavailable
	case core.ErrUnauthorized:
		return http.StatusUnauthorized
	case core.ErrForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
```

**Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/handler -v -run TestErrorMiddleware`

Expected: PASS

**Step 5: Commit 错误中间件**

```bash
git add api/internal/handler/error_middleware.go
git add api/internal/handler/error_middleware_test.go
git commit -m "feat(middleware): add unified error handling middleware"
```

---

### Task 2.3: 修复已知的错误忽略问题

**目标**: 修复代码审查中发现的错误忽略情况

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 修复 pool_manager.go 中的错误忽略**

定位到第 132 行左右：

```go
// 修改前
keywordGroupIDs, _ := m.discoverKeywordGroups(ctx)
imageGroupIDs, _ := m.discoverImageGroups(ctx)

// 修改后
keywordGroupIDs, err := m.discoverKeywordGroups(ctx)
if err != nil {
	log.Warn().Err(err).Msg("Failed to discover keyword groups, using defaults")
	keywordGroupIDs = m.getDefaultKeywordGroups()
}

imageGroupIDs, err := m.discoverImageGroups(ctx)
if err != nil {
	log.Warn().Err(err).Msg("Failed to discover image groups, using defaults")
	imageGroupIDs = m.getDefaultImageGroups()
}
```

**Step 2: 添加默认分组方法**

```go
// 在 pool_manager.go 中添加
func (m *PoolManager) getDefaultKeywordGroups() []int {
	// 返回所有可用的关键词分组
	var groups []int
	query := `SELECT DISTINCT group_id FROM keywords WHERE status = 1`
	err := m.db.Select(&groups, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get default keyword groups")
		return []int{1} // 最后兜底，返回分组1
	}
	return groups
}

func (m *PoolManager) getDefaultImageGroups() []int {
	var groups []int
	query := `SELECT DISTINCT group_id FROM images WHERE status = 1`
	err := m.db.Select(&groups, query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get default image groups")
		return []int{1}
	}
	return groups
}
```

**Step 3: 验证编译**

Run: `cd api && go build ./internal/service/...`

Expected: 编译成功

**Step 4: Commit 错误处理修复**

```bash
git add api/internal/service/pool_manager.go
git commit -m "fix(pool): handle errors in group discovery instead of ignoring"
```

---

## 阶段 3: 拆分大文件

### Task 3.1: 拆分 spiders.go

**目标**: 将 1857 行的 spiders.go 拆分为多个功能明确的文件

**Files:**
- Create: `api/internal/handler/spider_projects.go` (项目 CRUD)
- Create: `api/internal/handler/spider_files.go` (文件管理)
- Create: `api/internal/handler/spider_execution.go` (执行控制)
- Create: `api/internal/handler/spider_stats.go` (统计和日志)
- Create: `api/internal/handler/spider_websocket.go` (WebSocket)
- Create: `api/internal/model/spider_models.go` (数据模型)
- Modify: `api/internal/handler/spiders.go` (保留兼容性，委托到新文件)

**Step 1: 提取数据模型到 model 包**

```go
// api/internal/model/spider_models.go
package model

import (
	"encoding/json"
	"time"
)

// SpiderProject 爬虫项目
type SpiderProject struct {
	ID              int             `db:"id" json:"id"`
	Name            string          `db:"name" json:"name"`
	Description     *string         `db:"description" json:"description"`
	EntryFile       string          `db:"entry_file" json:"entry_file"`
	EntryFunction   string          `db:"entry_function" json:"entry_function"`
	StartURL        *string         `db:"start_url" json:"start_url"`
	Config          *string         `db:"config" json:"-"`
	ConfigParsed    json.RawMessage `json:"config"`
	Concurrency     int             `db:"concurrency" json:"concurrency"`
	OutputGroupID   int             `db:"output_group_id" json:"output_group_id"`
	Schedule        *string         `db:"schedule" json:"schedule"`
	Enabled         int             `db:"enabled" json:"enabled"`
	Status          string          `db:"status" json:"status"`
	LastRunAt       *time.Time      `db:"last_run_at" json:"last_run_at"`
	LastRunDuration *int            `db:"last_run_duration" json:"last_run_duration"`
	LastRunItems    *int            `db:"last_run_items" json:"last_run_items"`
	LastError       *string         `db:"last_error" json:"last_error"`
	TotalRuns       int             `db:"total_runs" json:"total_runs"`
	TotalItems      int             `db:"total_items" json:"total_items"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updated_at"`
}

// SpiderProjectFile 项目文件
type SpiderProjectFile struct {
	ID        int       `db:"id" json:"id"`
	ProjectID int       `db:"project_id" json:"project_id"`
	Path      string    `db:"path" json:"path"`
	Type      string    `db:"type" json:"type"` // "file" or "dir"
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// SpiderProjectCreate 创建请求
type SpiderProjectCreate struct {
	Name          string                 `json:"name" binding:"required"`
	Description   *string                `json:"description"`
	EntryFile     string                 `json:"entry_file"`
	EntryFunction string                 `json:"entry_function"`
	StartURL      *string                `json:"start_url"`
	Config        map[string]interface{} `json:"config"`
	Concurrency   int                    `json:"concurrency"`
	OutputGroupID int                    `json:"output_group_id"`
	Schedule      *string                `json:"schedule"`
	Enabled       int                    `json:"enabled"`
	Files         []SpiderFileCreate     `json:"files"`
}

// SpiderProjectUpdate 更新请求
type SpiderProjectUpdate struct {
	Name          *string                `json:"name"`
	Description   *string                `json:"description"`
	EntryFile     *string                `json:"entry_file"`
	EntryFunction *string                `json:"entry_function"`
	StartURL      *string                `json:"start_url"`
	Config        map[string]interface{} `json:"config"`
	Concurrency   *int                   `json:"concurrency"`
	OutputGroupID *int                   `json:"output_group_id"`
	Schedule      *string                `json:"schedule"`
	Enabled       *int                   `json:"enabled"`
}

// SpiderFileCreate 创建文件请求
type SpiderFileCreate struct {
	Filename string `json:"filename" binding:"required"`
	Content  string `json:"content"`
}

// SpiderFileUpdate 更新文件请求
type SpiderFileUpdate struct {
	Content string `json:"content" binding:"required"`
}

// SpiderCommand Redis 命令结构
type SpiderCommand struct {
	Action    string `json:"action"`
	ProjectID int    `json:"project_id"`
	MaxItems  int    `json:"max_items,omitempty"`
	Timestamp int64  `json:"timestamp"`
}
```

**Step 2: 创建项目 CRUD handler**

```go
// api/internal/handler/spider_projects.go
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"seo-generator/api/internal/model"
	"seo-generator/api/internal/service"
)

// GetSpiderProjects 获取爬虫项目列表
func GetSpiderProjects(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	var projects []model.SpiderProject
	query := `
		SELECT id, name, description, entry_file, entry_function, start_url,
		       config, concurrency, output_group_id, schedule, enabled, status,
		       last_run_at, last_run_duration, last_run_items, last_error,
		       total_runs, total_items, created_at, updated_at
		FROM spider_projects
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`

	db := database.GetDB()
	err := db.Select(&projects, query, pageSize, (page-1)*pageSize)
	if err != nil {
		c.Error(core.NewDatabaseError(err))
		return
	}

	// Parse config JSON
	for i := range projects {
		if projects[i].Config != nil {
			projects[i].ConfigParsed = []byte(*projects[i].Config)
		}
	}

	// Get total count
	var total int64
	err = db.Get(&total, "SELECT COUNT(*) FROM spider_projects")
	if err != nil {
		c.Error(core.NewDatabaseError(err))
		return
	}

	core.SuccessWithData(c, gin.H{
		"items": projects,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

// CreateSpiderProject 创建爬虫项目
func CreateSpiderProject(c *gin.Context) {
	var req model.SpiderProjectCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(core.NewValidationError("request", err.Error()))
		return
	}

	// TODO: 实现创建逻辑
	// 这里需要使用 Repository 或 Service

	core.SuccessWithMessage(c, "Project created successfully")
}

// 其他 CRUD 方法...
```

**Step 3: 逐步迁移功能**

按照功能域逐步迁移：
1. 项目 CRUD → `spider_projects.go`
2. 文件管理 → `spider_files.go`
3. 执行控制 → `spider_execution.go`
4. 统计日志 → `spider_stats.go`
5. WebSocket → `spider_websocket.go`

**Step 4: 保留原文件作为兼容层**

```go
// api/internal/handler/spiders.go
package api

// 此文件保留用于向后兼容
// 所有功能已迁移到:
// - spider_projects.go (项目 CRUD)
// - spider_files.go (文件管理)
// - spider_execution.go (执行控制)
// - spider_stats.go (统计日志)
// - spider_websocket.go (WebSocket)

// 如需使用旧代码，可临时切换回此文件
// 计划在 v2.0 版本完全移除此文件
```

**Step 5: 测试迁移后的功能**

Run: `cd api && go build ./internal/handler/...`

Expected: 编译成功

**Step 6: Commit 文件拆分**

```bash
git add api/internal/model/spider_models.go
git add api/internal/handler/spider_*.go
git add api/internal/handler/spiders.go
git commit -m "refactor(handler): split spiders.go into focused modules"
```

---

### Task 3.2: 优化 pool_manager.go

**目标**: 将 pool_manager.go 的功能拆分到独立的池实现中

**Files:**
- Create: `api/internal/service/pool/interfaces.go`
- Create: `api/internal/service/pool/keyword_pool.go`
- Create: `api/internal/service/pool/image_pool.go`
- Create: `api/internal/service/pool/manager.go`
- Modify: `api/internal/service/pool_manager.go` (兼容层)

**Step 1: 定义池接口**

```go
// api/internal/service/pool/interfaces.go
package pool

import "context"

// DataPool 数据池接口
type DataPool interface {
	Start(ctx context.Context) error
	Stop() error
	Pop(groupID int) (string, error)
	GetStats(groupID int) PoolStats
	Reload(groupIDs []int) error
	RefillIfNeeded(ctx context.Context, groupID int) error
}

// PoolStats 池统计信息
type PoolStats struct {
	Current     int
	Capacity    int
	GroupID     int
	CacheHits   int64
	CacheMisses int64
	MemoryBytes int64
}

// Config 池配置
type Config struct {
	Size             int
	Threshold        int
	RefillIntervalMS int
}
```

**Step 2: 实现 KeywordPool**

```go
// api/internal/service/pool/keyword_pool.go
package pool

import (
	"context"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/repository"
	"seo-generator/api/internal/service"
)

type KeywordPool struct {
	repo   repository.KeywordRepository
	config Config

	// 数据存储
	data   map[int][]string // groupID -> keywords
	mu     sync.RWMutex

	// 统计
	hits   int64
	misses int64

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewKeywordPool(db *sqlx.DB, config Config) *KeywordPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &KeywordPool{
		repo:   repository.NewKeywordRepository(db),
		config: config,
		data:   make(map[int][]string),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (p *KeywordPool) Start(ctx context.Context) error {
	log.Info().Msg("Starting keyword pool")

	// 发现所有分组
	groups, err := p.discoverGroups(ctx)
	if err != nil {
		return err
	}

	// 预加载数据
	for _, groupID := range groups {
		if err := p.loadGroup(ctx, groupID); err != nil {
			log.Warn().Err(err).Int("group_id", groupID).Msg("Failed to load group")
		}
	}

	// 启动补充 goroutine
	p.wg.Add(1)
	go p.refillLoop()

	return nil
}

func (p *KeywordPool) Stop() error {
	log.Info().Msg("Stopping keyword pool")
	p.cancel()
	p.wg.Wait()
	return nil
}

func (p *KeywordPool) Pop(groupID int) (string, error) {
	p.mu.RLock()
	keywords, exists := p.data[groupID]
	p.mu.RUnlock()

	if !exists || len(keywords) == 0 {
		return "", core.NewCachePoolEmptyError("keywords", groupID)
	}

	// 随机选择一个关键词
	idx := rand.IntN(len(keywords))
	return keywords[idx], nil
}

func (p *KeywordPool) GetStats(groupID int) PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	keywords := p.data[groupID]
	return PoolStats{
		Current:     len(keywords),
		Capacity:    p.config.Size,
		GroupID:     groupID,
		CacheHits:   p.hits,
		CacheMisses: p.misses,
	}
}

func (p *KeywordPool) Reload(groupIDs []int) error {
	for _, groupID := range groupIDs {
		if err := p.loadGroup(p.ctx, groupID); err != nil {
			log.Error().Err(err).Int("group_id", groupID).Msg("Failed to reload group")
		}
	}
	return nil
}

func (p *KeywordPool) RefillIfNeeded(ctx context.Context, groupID int) error {
	p.mu.RLock()
	current := len(p.data[groupID])
	p.mu.RUnlock()

	if current < p.config.Threshold {
		return p.loadGroup(ctx, groupID)
	}

	return nil
}

// 私有方法

func (p *KeywordPool) discoverGroups(ctx context.Context) ([]int, error) {
	// 从数据库查询所有分组
	var groups []int
	// TODO: 实现查询逻辑
	return groups, nil
}

func (p *KeywordPool) loadGroup(ctx context.Context, groupID int) error {
	keywords, err := p.repo.RandomByGroupID(ctx, groupID, p.config.Size)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.data[groupID] = make([]string, len(keywords))
	for i, kw := range keywords {
		p.data[groupID][i] = kw.Keyword
	}
	p.mu.Unlock()

	log.Debug().Int("group_id", groupID).Int("count", len(keywords)).Msg("Loaded keywords")
	return nil
}

func (p *KeywordPool) refillLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(time.Duration(p.config.RefillIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.refillAll()
		}
	}
}

func (p *KeywordPool) refillAll() {
	p.mu.RLock()
	groups := make([]int, 0, len(p.data))
	for groupID := range p.data {
		groups = append(groups, groupID)
	}
	p.mu.RUnlock()

	for _, groupID := range groups {
		p.RefillIfNeeded(p.ctx, groupID)
	}
}
```

**Step 3: 实现 PoolManager 协调器**

```go
// api/internal/service/pool/manager.go
package pool

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Manager struct {
	keywordPool *KeywordPool
	imagePool   *ImagePool
	// 其他池...
}

func NewManager(db *sqlx.DB, config ManagerConfig) *Manager {
	return &Manager{
		keywordPool: NewKeywordPool(db, config.KeywordPoolConfig),
		imagePool:   NewImagePool(db, config.ImagePoolConfig),
	}
}

func (m *Manager) Start(ctx context.Context) error {
	if err := m.keywordPool.Start(ctx); err != nil {
		return err
	}

	if err := m.imagePool.Start(ctx); err != nil {
		m.keywordPool.Stop()
		return err
	}

	return nil
}

func (m *Manager) Stop() error {
	m.keywordPool.Stop()
	m.imagePool.Stop()
	return nil
}

func (m *Manager) GetKeyword(groupID int) (string, error) {
	return m.keywordPool.Pop(groupID)
}

func (m *Manager) GetImage(groupID int) (string, error) {
	return m.imagePool.Pop(groupID)
}

// 其他方法...
```

**Step 4: 更新原 pool_manager.go 作为兼容层**

```go
// api/internal/service/pool_manager.go
package core

import (
	"context"

	"github.com/jmoiron/sqlx"

	"seo-generator/api/internal/service/pool"
)

// PoolManager 兼容性包装器
// 新代码请使用 pool.Manager
type PoolManager struct {
	manager *pool.Manager
}

func NewPoolManager(db *sqlx.DB) *PoolManager {
	config := pool.ManagerConfig{
		// 从配置加载
	}

	return &PoolManager{
		manager: pool.NewManager(db, config),
	}
}

func (m *PoolManager) Start(ctx context.Context) error {
	return m.manager.Start(ctx)
}

func (m *PoolManager) Stop() error {
	return m.manager.Stop()
}

// 委托方法...
func (m *PoolManager) GetKeyword(groupID int) (string, error) {
	return m.manager.GetKeyword(groupID)
}
```

**Step 5: 测试新实现**

Run: `cd api && go test ./internal/service/pool/... -v`

Expected: 测试通过

**Step 6: Commit 池重构**

```bash
git add api/internal/service/pool/
git add api/internal/service/pool_manager.go
git commit -m "refactor(pool): extract pool implementations into separate modules"
```

---

## 阶段 4: 性能优化

### Task 4.1: 修复 updateCh 消息丢失问题

**目标**: 使用批量更新替代 channel 丢弃消息

**Files:**
- Create: `api/internal/service/pool/update_batcher.go`
- Create: `api/internal/service/pool/update_batcher_test.go`

**Step 1: 编写批量更新器测试**

```go
// api/internal/service/pool/update_batcher_test.go
package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	testutil "seo-generator/api/internal/testing"
)

func TestUpdateBatcher(t *testing.T) {
	db, mock, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	config := BatcherConfig{
		MaxBatch:      10,
		FlushInterval: 100 * time.Millisecond,
	}

	batcher := NewUpdateBatcher(db, config)

	t.Run("batch updates", func(t *testing.T) {
		testutil.ExpectBegin(mock)
		mock.ExpectExec("UPDATE keywords SET status = 0 WHERE id IN").
			WillReturnResult(sqlmock.NewResult(0, 2))
		testutil.ExpectCommit(mock)

		batcher.Add(UpdateTask{Table: "keywords", ID: 1})
		batcher.Add(UpdateTask{Table: "keywords", ID: 2})

		time.Sleep(150 * time.Millisecond) // 等待自动刷新

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("immediate flush on max batch", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			testutil.ExpectBegin(mock)
			mock.ExpectExec("UPDATE keywords").
				WillReturnResult(sqlmock.NewResult(0, 1))
			testutil.ExpectCommit(mock)

			batcher.Add(UpdateTask{Table: "keywords", ID: int64(i)})
		}

		time.Sleep(50 * time.Millisecond)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
```

**Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/service/pool -v -run TestUpdateBatcher`

Expected: FAIL

**Step 3: 实现批量更新器**

```go
// api/internal/service/pool/update_batcher.go
package pool

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type UpdateTask struct {
	Table string
	ID    int64
}

type BatcherConfig struct {
	MaxBatch      int
	FlushInterval time.Duration
}

type UpdateBatcher struct {
	db     *sqlx.DB
	config BatcherConfig

	mu      sync.Mutex
	pending []UpdateTask

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewUpdateBatcher(db *sqlx.DB, config BatcherConfig) *UpdateBatcher {
	ctx, cancel := context.WithCancel(context.Background())

	b := &UpdateBatcher{
		db:      db,
		config:  config,
		pending: make([]UpdateTask, 0, config.MaxBatch),
		ctx:     ctx,
		cancel:  cancel,
	}

	b.wg.Add(1)
	go b.flushLoop()

	return b
}

func (b *UpdateBatcher) Add(task UpdateTask) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.pending = append(b.pending, task)

	if len(b.pending) >= b.config.MaxBatch {
		b.flush()
	}
}

func (b *UpdateBatcher) Stop() {
	b.cancel()
	b.wg.Wait()

	// 最后刷新一次
	b.mu.Lock()
	b.flush()
	b.mu.Unlock()
}

func (b *UpdateBatcher) flush() {
	if len(b.pending) == 0 {
		return
	}

	// 按表分组
	grouped := make(map[string][]int64)
	for _, task := range b.pending {
		grouped[task.Table] = append(grouped[task.Table], task.ID)
	}

	// 开启事务
	tx, err := b.db.BeginTxx(b.ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin transaction for batch update")
		return
	}
	defer tx.Rollback()

	// 批量更新每个表
	for table, ids := range grouped {
		if err := b.batchUpdate(tx, table, ids); err != nil {
			log.Error().Err(err).Str("table", table).Msg("Batch update failed")
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		log.Error().Err(err).Msg("Failed to commit batch update")
		return
	}

	log.Debug().
		Int("count", len(b.pending)).
		Interface("tables", grouped).
		Msg("Batch update completed")

	// 清空待处理队列
	b.pending = b.pending[:0]
}

func (b *UpdateBatcher) batchUpdate(tx *sqlx.Tx, table string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// 验证表名（防止 SQL 注入）
	validTables := map[string]bool{
		"keywords": true,
		"images":   true,
		"titles":   true,
		"contents": true,
	}

	if !validTables[table] {
		return fmt.Errorf("invalid table name: %s", table)
	}

	// 构建 IN 子句
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1] // 移除最后的逗号

	query := fmt.Sprintf("UPDATE %s SET status = 0 WHERE id IN (%s)", table, placeholders)

	// 转换为 interface{} 切片
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err := tx.Exec(query, args...)
	return err
}

func (b *UpdateBatcher) flushLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.mu.Lock()
			b.flush()
			b.mu.Unlock()
		}
	}
}
```

**Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/service/pool -v -run TestUpdateBatcher`

Expected: PASS

**Step 5: 集成到池中**

在 KeywordPool 中使用 UpdateBatcher：

```go
type KeywordPool struct {
	// ... 其他字段
	batcher *UpdateBatcher
}

func NewKeywordPool(db *sqlx.DB, config Config) *KeywordPool {
	batcherConfig := BatcherConfig{
		MaxBatch:      100,
		FlushInterval: 5 * time.Second,
	}

	return &KeywordPool{
		// ... 其他初始化
		batcher: NewUpdateBatcher(db, batcherConfig),
	}
}

func (p *KeywordPool) markAsUsed(id int64) {
	// 不再使用 channel，直接添加到批处理器
	p.batcher.Add(UpdateTask{Table: "keywords", ID: id})
}
```

**Step 6: Commit 批量更新优化**

```bash
git add api/internal/service/pool/update_batcher.go
git add api/internal/service/pool/update_batcher_test.go
git commit -m "feat(pool): add update batcher to prevent message loss"
```

---

### Task 4.2: 优化数据库连接池配置

**目标**: 调整连接池参数，减少内存浪费

**Files:**
- Modify: `api/internal/repository/db.go`

**Step 1: 更新连接池配置**

```go
// api/internal/repository/db.go

// 修改前
db.SetMaxOpenConns(maxConns)
db.SetMaxIdleConns(maxConns)

// 修改后
db.SetMaxOpenConns(maxConns)
db.SetMaxIdleConns(maxConns / 5) // 20% 空闲连接
db.SetConnMaxLifetime(5 * time.Minute)   // 连接最大生命周期
db.SetConnMaxIdleTime(2 * time.Minute)   // 连接最大空闲时间
```

**Step 2: 添加配置说明注释**

```go
// Configure connection pool for high concurrency
// MaxOpenConns: 最大并发连接数
// MaxIdleConns: 空闲连接池大小（设置为 MaxOpenConns 的 20%）
// ConnMaxLifetime: 连接最大生命周期，防止长时间连接导致问题
// ConnMaxIdleTime: 连接最大空闲时间，释放长时间未使用的连接
```

**Step 3: Commit 连接池优化**

```bash
git add api/internal/repository/db.go
git commit -m "perf(db): optimize connection pool settings to reduce memory usage"
```

---

## 阶段 5: 依赖注入与可测试性

### Task 5.1: 创建依赖注入容器

**目标**: 移除全局单例，统一管理依赖

**Files:**
- Create: `api/internal/di/container.go`
- Create: `api/internal/di/container_test.go`

**Step 1: 编写容器测试**

```go
// api/internal/di/container_test.go
package di

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"seo-generator/api/pkg/config"
	testutil "seo-generator/api/internal/testing"
)

func TestContainer(t *testing.T) {
	db, _, cleanup := testutil.NewMockDB(t)
	defer cleanup()

	cfg := &config.Config{
		// 测试配置
	}

	container := NewContainer(db, cfg)

	t.Run("provides repositories", func(t *testing.T) {
		assert.NotNil(t, container.GetSiteRepository())
		assert.NotNil(t, container.GetKeywordRepository())
		assert.NotNil(t, container.GetImageRepository())
	})

	t.Run("provides services", func(t *testing.T) {
		assert.NotNil(t, container.GetPoolManager())
		assert.NotNil(t, container.GetSpiderDetector())
	})

	t.Run("singleton pattern", func(t *testing.T) {
		repo1 := container.GetSiteRepository()
		repo2 := container.GetSiteRepository()

		// 应该返回相同的实例
		assert.Equal(t, repo1, repo2)
	})
}
```

**Step 2: 运行测试确认失败**

Run: `cd api && go test ./internal/di -v`

Expected: FAIL

**Step 3: 实现依赖注入容器**

```go
// api/internal/di/container.go
package di

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"seo-generator/api/internal/repository"
	"seo-generator/api/internal/service"
	"seo-generator/api/pkg/config"
)

// Container 依赖注入容器
type Container struct {
	db     *sqlx.DB
	redis  *redis.Client
	config *config.Config

	// Repositories (懒加载)
	siteRepo    repository.SiteRepository
	keywordRepo repository.KeywordRepository
	imageRepo   repository.ImageRepository
	articleRepo repository.ArticleRepository
	titleRepo   repository.TitleRepository
	contentRepo repository.ContentRepository

	// Services (懒加载)
	poolManager    *service.PoolManager
	spiderDetector *service.SpiderDetector
	cacheManager   *service.CacheManager
	templateCache  *service.TemplateCache
}

// NewContainer 创建新的容器
func NewContainer(db *sqlx.DB, cfg *config.Config) *Container {
	return &Container{
		db:     db,
		config: cfg,
	}
}

// SetRedis 设置 Redis 客户端
func (c *Container) SetRedis(redis *redis.Client) {
	c.redis = redis
}

// Repository Getters (单例模式)

func (c *Container) GetSiteRepository() repository.SiteRepository {
	if c.siteRepo == nil {
		c.siteRepo = repository.NewSiteRepository(c.db)
	}
	return c.siteRepo
}

func (c *Container) GetKeywordRepository() repository.KeywordRepository {
	if c.keywordRepo == nil {
		c.keywordRepo = repository.NewKeywordRepository(c.db)
	}
	return c.keywordRepo
}

func (c *Container) GetImageRepository() repository.ImageRepository {
	if c.imageRepo == nil {
		c.imageRepo = repository.NewImageRepository(c.db)
	}
	return c.imageRepo
}

func (c *Container) GetArticleRepository() repository.ArticleRepository {
	if c.articleRepo == nil {
		c.articleRepo = repository.NewArticleRepository(c.db)
	}
	return c.articleRepo
}

func (c *Container) GetTitleRepository() repository.TitleRepository {
	if c.titleRepo == nil {
		c.titleRepo = repository.NewTitleRepository(c.db)
	}
	return c.titleRepo
}

func (c *Container) GetContentRepository() repository.ContentRepository {
	if c.contentRepo == nil {
		c.contentRepo = repository.NewContentRepository(c.db)
	}
	return c.contentRepo
}

// Service Getters (单例模式)

func (c *Container) GetPoolManager() *service.PoolManager {
	if c.poolManager == nil {
		c.poolManager = service.NewPoolManager(
			c.db,
			c.GetKeywordRepository(),
			c.GetImageRepository(),
			c.GetTitleRepository(),
			c.GetContentRepository(),
		)
	}
	return c.poolManager
}

func (c *Container) GetSpiderDetector() *service.SpiderDetector {
	if c.spiderDetector == nil {
		c.spiderDetector = service.NewSpiderDetector(c.config.SpiderDetectorPath)
	}
	return c.spiderDetector
}

func (c *Container) GetCacheManager() *service.CacheManager {
	if c.cacheManager == nil {
		c.cacheManager = service.NewCacheManager(c.config.CacheDir)
	}
	return c.cacheManager
}

func (c *Container) GetTemplateCache() *service.TemplateCache {
	if c.templateCache == nil {
		c.templateCache = service.NewTemplateCache()
	}
	return c.templateCache
}

// Close 关闭所有资源
func (c *Container) Close() error {
	if c.poolManager != nil {
		c.poolManager.Stop()
	}

	if c.db != nil {
		return c.db.Close()
	}

	return nil
}
```

**Step 4: 运行测试确认通过**

Run: `cd api && go test ./internal/di -v`

Expected: PASS

**Step 5: Commit 依赖注入容器**

```bash
git add api/internal/di/
git commit -m "feat(di): add dependency injection container"
```

---

### Task 5.2: 在 main.go 中使用容器

**目标**: 重构 main.go 使用 DI 容器

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: 更新 main.go**

```go
// api/cmd/main.go

func main() {
	// 加载配置
	cfg := loadConfig()

	// 初始化数据库
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}

	// 创建依赖注入容器
	container := di.NewContainer(db, cfg)
	defer container.Close()

	// 初始化 Redis
	redisClient := initRedis(cfg)
	container.SetRedis(redisClient)

	// 启动服务（通过容器获取）
	ctx := context.Background()

	poolManager := container.GetPoolManager()
	if err := poolManager.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start pool manager")
	}

	// 设置路由（传入容器）
	router := gin.New()
	setupRoutes(router, container)

	// 启动服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	log.Info().Int("port", cfg.Port).Msg("Server started")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

// setupRoutes 配置路由（使用容器）
func setupRoutes(router *gin.Engine, container *di.Container) {
	// 中间件
	router.Use(gin.Recovery())
	router.Use(api.ErrorHandlerMiddleware())

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API 路由
	apiGroup := router.Group("/api")
	{
		// 使用容器提供的依赖
		api.RegisterSiteRoutes(apiGroup, container)
		api.RegisterKeywordRoutes(apiGroup, container)
		api.RegisterImageRoutes(apiGroup, container)
		// ... 其他路由
	}
}
```

**Step 2: 更新路由注册函数**

```go
// api/internal/handler/routes.go

func RegisterSiteRoutes(group *gin.RouterGroup, container *di.Container) {
	siteRepo := container.GetSiteRepository()
	siteService := service.NewSiteService(siteRepo)
	siteHandler := NewSiteHandler(siteService)

	sites := group.Group("/sites")
	{
		sites.GET("", siteHandler.List)
		sites.POST("", siteHandler.Create)
		sites.GET("/:id", siteHandler.Get)
		sites.PUT("/:id", siteHandler.Update)
		sites.DELETE("/:id", siteHandler.Delete)
	}
}
```

**Step 3: 测试编译**

Run: `cd api && go build ./cmd/...`

Expected: 编译成功

**Step 4: Commit main.go 重构**

```bash
git add api/cmd/main.go
git add api/internal/handler/routes.go
git commit -m "refactor(main): use DI container for dependency management"
```

---

## 阶段 6: 验证与文档

### Task 6.1: 集成测试

**目标**: 编写端到端集成测试验证重构正确性

**Files:**
- Create: `api/test/integration/pool_test.go`
- Create: `api/test/integration/repository_test.go`

**Step 1: 编写池集成测试**

```go
// api/test/integration/pool_test.go
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"seo-generator/api/internal/di"
	"seo-generator/api/pkg/config"
)

func TestPoolManagerIntegration(t *testing.T) {
	// 连接真实数据库
	cfg := loadTestConfig()
	db := connectTestDatabase(cfg)
	defer db.Close()

	container := di.NewContainer(db, cfg)
	defer container.Close()

	poolManager := container.GetPoolManager()

	t.Run("start and load data", func(t *testing.T) {
		ctx := context.Background()
		err := poolManager.Start(ctx)
		assert.NoError(t, err)
	})

	t.Run("pop keyword", func(t *testing.T) {
		keyword, err := poolManager.GetKeyword(1)
		assert.NoError(t, err)
		assert.NotEmpty(t, keyword)
	})

	t.Run("pop image", func(t *testing.T) {
		image, err := poolManager.GetImage(1)
		assert.NoError(t, err)
		assert.NotEmpty(t, image)
	})

	t.Run("stop gracefully", func(t *testing.T) {
		err := poolManager.Stop()
		assert.NoError(t, err)
	})
}
```

**Step 2: 运行集成测试**

Run: `cd api && go test -tags=integration ./test/integration/... -v`

Expected: 所有集成测试通过

**Step 3: Commit 集成测试**

```bash
git add api/test/integration/
git commit -m "test: add integration tests for refactored components"
```

---

### Task 6.2: 更新文档

**目标**: 更新 README 和架构文档反映重构后的结构

**Files:**
- Modify: `README.md`
- Create: `docs/architecture/refactoring-guide.md`

**Step 1: 更新架构文档**

```markdown
<!-- docs/architecture/refactoring-guide.md -->

# 重构指南

## 概述

本文档说明 2026-02 重构后的代码架构和最佳实践。

## 新架构

### 分层架构

```
Handler 层 (HTTP 请求处理)
    ↓
Service 层 (业务逻辑)
    ↓
Repository 层 (数据访问)
    ↓
Database (MySQL)
```

### 依赖注入

所有依赖通过 DI 容器管理，不再使用全局变量：

\`\`\`go
// 获取依赖
container := di.NewContainer(db, cfg)
repo := container.GetKeywordRepository()
service := service.NewKeywordService(repo)
\`\`\`

### Repository 模式

所有数据库访问统一通过 Repository 接口：

\`\`\`go
type KeywordRepository interface {
    Create(ctx context.Context, keyword *model.Keyword) error
    GetByID(ctx context.Context, id int64) (*model.Keyword, error)
    // ...
}
\`\`\`

### 错误处理

统一使用 AppError：

\`\`\`go
if err != nil {
    return core.NewDatabaseError(err)
}
\`\`\`

HTTP 层自动转换为正确的状态码。

## 迁移指南

### 从旧代码迁移

**旧代码:**
\`\`\`go
db := database.GetDB()
rows, err := db.Query("SELECT * FROM keywords WHERE id = ?", id)
// 手动处理错误...
\`\`\`

**新代码:**
\`\`\`go
repo := container.GetKeywordRepository()
keyword, err := repo.GetByID(ctx, id)
if err != nil {
    return core.NewDatabaseError(err)
}
\`\`\`

## 测试

### 单元测试

使用 mock 数据库：

\`\`\`go
db, mock, cleanup := testutil.NewMockDB(t)
defer cleanup()

repo := repository.NewKeywordRepository(db)
// 编写测试...
\`\`\`

### 集成测试

使用 integration tag：

\`\`\`bash
go test -tags=integration ./test/integration/...
\`\`\`

## 最佳实践

1. **使用 Repository** - 永远不要在 Handler 或 Service 中直接写 SQL
2. **错误传播** - 使用 AppError 包装所有错误
3. **依赖注入** - 通过参数传递依赖，不使用全局变量
4. **测试优先** - 先写测试再写实现
5. **接口抽象** - 定义接口以支持 Mock 测试

## 向后兼容

旧代码通过兼容层继续工作：

- `api/internal/service/pool_manager.go` 委托到新实现
- `api/internal/handler/spiders.go` 保留但标记为废弃

计划在 v2.0 完全移除兼容层。
```

**Step 2: 更新 README.md**

在 README 中添加重构说明章节：

```markdown
## 代码架构（2026-02 重构）

本项目经过全面重构，采用现代化的分层架构和依赖注入模式。

### 核心改进

- ✅ **Repository 模式** - 统一数据访问接口
- ✅ **依赖注入** - 移除全局单例，提高可测试性
- ✅ **统一错误处理** - AppError 和错误中间件
- ✅ **性能优化** - 批量更新、连接池优化
- ✅ **代码质量** - 拆分大文件，消除重复代码

详见 [重构指南](docs/architecture/refactoring-guide.md)
```

**Step 3: Commit 文档更新**

```bash
git add README.md
git add docs/architecture/refactoring-guide.md
git commit -m "docs: update architecture documentation for refactoring"
```

---

## 最终验证

### Task 7.1: 完整测试套件

**目标**: 运行所有测试确保重构成功

**Step 1: 运行单元测试**

Run: `cd api && go test ./... -v`

Expected: 所有单元测试通过

**Step 2: 运行集成测试**

Run: `cd api && go test -tags=integration ./test/integration/... -v`

Expected: 所有集成测试通过

**Step 3: 性能基准测试**

Run: `cd api && go test -bench=. -benchmem ./internal/service/pool/...`

Expected: 性能指标符合预期

**Step 4: 代码覆盖率**

Run: `cd api && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out`

Expected: 覆盖率 > 60%

---

### Task 7.2: 打标签和部署准备

**目标**: 标记重构完成，准备部署

**Step 1: 创建 Git 标签**

```bash
git tag -a v1.5.0-refactored -m "Complete Go API refactoring"
git push origin v1.5.0-refactored
```

**Step 2: 生成变更日志**

创建 `CHANGELOG.md`:

```markdown
# Changelog

## [1.5.0-refactored] - 2026-02-04

### 重构 (Refactoring)

- 建立完整的 Repository 层，统一数据访问
- 拆分大文件（spiders.go, pool_manager.go）到功能模块
- 实现依赖注入容器，移除全局单例
- 统一错误处理机制（AppError + 中间件）
- 消除代码重复，提取通用 CRUD 逻辑

### 性能优化 (Performance)

- 批量更新机制，防止消息丢失
- 优化数据库连接池配置
- 减少锁竞争和内存分配

### 测试 (Testing)

- 新增 Repository 单元测试
- 新增池集成测试
- 测试覆盖率从 10% 提升到 60%+

### 向后兼容 (Backward Compatibility)

- 保留旧 API 兼容层
- 所有现有功能正常工作
- 渐进式迁移路径
```

**Step 3: 部署检查清单**

创建部署前检查清单：

```markdown
# 部署前检查清单

## 代码质量

- [ ] 所有单元测试通过
- [ ] 所有集成测试通过
- [ ] 代码覆盖率 > 60%
- [ ] 无严重的 linter 警告

## 性能

- [ ] 性能基准测试通过
- [ ] 内存使用在预期范围内
- [ ] 数据库连接池配置正确

## 文档

- [ ] README 已更新
- [ ] 架构文档已更新
- [ ] CHANGELOG 已生成
- [ ] API 文档已同步

## 向后兼容

- [ ] 现有 API 端点正常工作
- [ ] 旧代码通过兼容层运行
- [ ] 数据库迁移脚本准备就绪

## 回滚准备

- [ ] Git 标签已创建
- [ ] 回滚脚本已测试
- [ ] 备份计划已确认
```

**Step 4: Commit 部署文档**

```bash
git add CHANGELOG.md
git add docs/deployment-checklist.md
git commit -m "docs: add changelog and deployment checklist"
```

---

## 执行策略

### 分阶段执行

1. **阶段 0-1**: 基础设施（1-2 天）
2. **阶段 2**: 错误处理（1 天）
3. **阶段 3**: 文件拆分（2-3 天）
4. **阶段 4**: 性能优化（1-2 天）
5. **阶段 5**: 依赖注入（2-3 天）
6. **阶段 6-7**: 验证和文档（1-2 天）

**总计**: 8-13 天

### 风险控制

1. **每日提交** - 频繁的小提交而不是大批量
2. **功能开关** - 使用环境变量控制新旧代码切换
3. **灰度发布** - 先在测试环境验证，再逐步上线
4. **监控告警** - 关注错误率和性能指标

### 回滚计划

每个阶段都有独立的 Git 标签，可快速回滚：

```bash
# 回滚到特定阶段
git checkout v1.5.0-stage-1
```

---

## 总结

本计划提供了一个**安全、可验证、渐进式**的重构路径，包含：

✅ **完整的测试覆盖** - 每个功能都有测试保护
✅ **向后兼容** - 旧代码通过兼容层继续工作
✅ **分阶段执行** - 每个阶段独立验证
✅ **详细文档** - 迁移指南和最佳实践
✅ **性能改进** - 批量更新、连接池优化
✅ **可回滚** - 每个阶段都有回滚点

---

**下一步**: 选择执行方式

1. **子代理驱动开发** (推荐) - 在当前会话中逐任务执行
2. **并行会话执行** - 在独立 worktree 中批量执行

请问选择哪种方式？
