// Package di provides dependency injection container for the application.
// It manages the lifecycle of repositories, services, and other dependencies.
package di

import (
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"seo-generator/api/internal/repository"
	"seo-generator/api/pkg/config"
)

// Container is the dependency injection container that manages
// all application dependencies with lazy initialization and singleton pattern.
type Container struct {
	db     *sqlx.DB
	redis  *redis.Client
	config *config.Config

	mu sync.RWMutex

	// Repositories (lazily initialized singletons)
	siteRepo    repository.SiteRepository
	keywordRepo repository.KeywordRepository
	imageRepo   repository.ImageRepository
	articleRepo repository.ArticleRepository
	titleRepo   repository.TitleRepository
	contentRepo repository.ContentRepository
}

// NewContainer creates a new dependency injection container.
func NewContainer(db *sqlx.DB, cfg *config.Config) *Container {
	return &Container{
		db:     db,
		config: cfg,
	}
}

// SetRedis sets the Redis client for the container.
func (c *Container) SetRedis(redis *redis.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.redis = redis
}

// GetDB returns the database connection.
func (c *Container) GetDB() *sqlx.DB {
	return c.db
}

// GetRedis returns the Redis client (may be nil if not configured).
func (c *Container) GetRedis() *redis.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.redis
}

// GetConfig returns the application configuration.
func (c *Container) GetConfig() *config.Config {
	return c.config
}

// Repository Getters (singleton pattern with lazy initialization)

// GetSiteRepository returns the site repository singleton.
func (c *Container) GetSiteRepository() repository.SiteRepository {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.siteRepo == nil {
		c.siteRepo = repository.NewSiteRepository(c.db)
	}
	return c.siteRepo
}

// GetKeywordRepository returns the keyword repository singleton.
func (c *Container) GetKeywordRepository() repository.KeywordRepository {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.keywordRepo == nil {
		c.keywordRepo = repository.NewKeywordRepository(c.db)
	}
	return c.keywordRepo
}

// GetImageRepository returns the image repository singleton.
func (c *Container) GetImageRepository() repository.ImageRepository {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.imageRepo == nil {
		c.imageRepo = repository.NewImageRepository(c.db)
	}
	return c.imageRepo
}

// GetArticleRepository returns the article repository singleton.
func (c *Container) GetArticleRepository() repository.ArticleRepository {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.articleRepo == nil {
		c.articleRepo = repository.NewArticleRepository(c.db)
	}
	return c.articleRepo
}

// GetTitleRepository returns the title repository singleton.
func (c *Container) GetTitleRepository() repository.TitleRepository {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.titleRepo == nil {
		c.titleRepo = repository.NewTitleRepository(c.db)
	}
	return c.titleRepo
}

// GetContentRepository returns the content repository singleton.
func (c *Container) GetContentRepository() repository.ContentRepository {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.contentRepo == nil {
		c.contentRepo = repository.NewContentRepository(c.db)
	}
	return c.contentRepo
}

// Close releases all resources held by the container.
// Note: The container does not own the database connection,
// so it does not close it. The caller is responsible for closing the database.
func (c *Container) Close() {
	// Currently, repositories don't need explicit cleanup.
	// This method is provided for future extensibility
	// (e.g., closing pooled connections, stopping background workers).
}
