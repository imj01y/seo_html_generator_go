package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	ErrPoolEmpty = errors.New("pool is empty")
)

// PoolItem represents an item in the pool
type PoolItem struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
}

// UpdateTask represents a status update task
type UpdateTask struct {
	Table string
	ID    int64
}

// PoolConsumer consumes titles and contents from Redis pools
type PoolConsumer struct {
	redis    *redis.Client
	db       *sqlx.DB
	updateCh chan UpdateTask
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewPoolConsumer creates a new pool consumer
func NewPoolConsumer(redisClient *redis.Client, db *sqlx.DB) *PoolConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolConsumer{
		redis:    redisClient,
		db:       db,
		updateCh: make(chan UpdateTask, 1000),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the async update worker
func (c *PoolConsumer) Start() {
	c.wg.Add(1)
	go c.updateWorker()
	log.Info().Msg("PoolConsumer started")
}

// Stop stops the pool consumer gracefully
func (c *PoolConsumer) Stop() {
	c.cancel()
	close(c.updateCh)
	c.wg.Wait()
	log.Info().Msg("PoolConsumer stopped")
}

// updateWorker processes status update tasks
func (c *PoolConsumer) updateWorker() {
	defer c.wg.Done()

	for task := range c.updateCh {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.processUpdate(task)
		}
	}
}

// processUpdate updates the status of a consumed item
func (c *PoolConsumer) processUpdate(task UpdateTask) {
	query := fmt.Sprintf("UPDATE %s SET status = 0 WHERE id = ?", task.Table)
	_, err := c.db.ExecContext(c.ctx, query, task.ID)
	if err != nil {
		log.Warn().Err(err).
			Str("table", task.Table).
			Int64("id", task.ID).
			Msg("Failed to update status")
	}
}

// Pop retrieves and removes an item from the pool
func (c *PoolConsumer) Pop(ctx context.Context, poolType string, groupID int) (string, error) {
	if c.redis == nil {
		return "", errors.New("redis client is nil")
	}

	key := fmt.Sprintf("%s:pool:%d", poolType, groupID)

	// RPOP from Redis
	data, err := c.redis.RPop(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrPoolEmpty
	}
	if err != nil {
		return "", fmt.Errorf("redis rpop failed: %w", err)
	}

	// Parse JSON
	var item PoolItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return "", fmt.Errorf("json unmarshal failed: %w", err)
	}

	// Async update status
	select {
	case c.updateCh <- UpdateTask{Table: poolType, ID: item.ID}:
	default:
		log.Warn().Str("table", poolType).Int64("id", item.ID).Msg("Update channel full, dropping task")
	}

	return item.Text, nil
}

// PopWithFallback tries Redis first, falls back to DB on failure
func (c *PoolConsumer) PopWithFallback(ctx context.Context, poolType string, groupID int) (string, error) {
	// Try Redis first
	text, err := c.Pop(ctx, poolType, groupID)
	if err == nil {
		return text, nil
	}

	// Fallback to DB on Redis errors
	if err == ErrPoolEmpty || errors.Is(err, redis.Nil) || c.redis == nil {
		log.Debug().Str("pool", poolType).Int("group", groupID).Msg("Falling back to DB")
		return c.fallbackFromDB(ctx, poolType, groupID)
	}

	return "", err
}

// fallbackFromDB queries DB directly when Redis is unavailable
func (c *PoolConsumer) fallbackFromDB(ctx context.Context, poolType string, groupID int) (string, error) {
	column := "title"
	if poolType == "contents" {
		column = "content"
	}

	query := fmt.Sprintf(`
		SELECT id, %s as text FROM %s
		WHERE group_id = ? AND status = 1
		ORDER BY batch_id DESC, id ASC
		LIMIT 1
	`, column, poolType)

	var item PoolItem
	if err := c.db.GetContext(ctx, &item, query, groupID); err != nil {
		return "", fmt.Errorf("db fallback failed: %w", err)
	}

	// Async update status
	select {
	case c.updateCh <- UpdateTask{Table: poolType, ID: item.ID}:
	default:
		log.Warn().Str("table", poolType).Int64("id", item.ID).Msg("Update channel full, dropping task")
	}

	return item.Text, nil
}

// GetPoolLength returns the current length of a pool
func (c *PoolConsumer) GetPoolLength(ctx context.Context, poolType string, groupID int) (int64, error) {
	if c.redis == nil {
		return 0, errors.New("redis client is nil")
	}
	key := fmt.Sprintf("%s:pool:%d", poolType, groupID)
	return c.redis.LLen(ctx, key).Result()
}
