package core

import (
	"context"
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
