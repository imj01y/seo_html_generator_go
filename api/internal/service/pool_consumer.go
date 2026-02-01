package core

import (
	"context"
	"errors"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
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
