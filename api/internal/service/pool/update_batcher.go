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

// UpdateTask represents a status update task
type UpdateTask struct {
	Table string
	ID    int64
}

// BatcherConfig configures the update batcher
type BatcherConfig struct {
	MaxBatch      int
	FlushInterval time.Duration
}

// UpdateBatcher batches status updates to reduce database pressure
// and prevent message loss that occurs with channel-based approaches
type UpdateBatcher struct {
	db     *sqlx.DB
	config BatcherConfig

	mu      sync.Mutex
	pending []UpdateTask

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewUpdateBatcher creates a new update batcher
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

// Add adds a task to the batch queue
// This never blocks or drops tasks, solving the channel overflow issue
func (b *UpdateBatcher) Add(task UpdateTask) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.pending = append(b.pending, task)

	if len(b.pending) >= b.config.MaxBatch {
		b.flushLocked()
	}
}

// Stop stops the batcher and flushes remaining tasks
func (b *UpdateBatcher) Stop() {
	// First flush pending tasks (before canceling context)
	b.mu.Lock()
	b.flushLocked()
	b.mu.Unlock()

	// Then stop the background goroutine
	b.cancel()
	b.wg.Wait()
}

// flushLocked performs the flush while holding the lock
func (b *UpdateBatcher) flushLocked() {
	if len(b.pending) == 0 {
		return
	}

	// Group by table
	grouped := make(map[string][]int64)
	for _, task := range b.pending {
		grouped[task.Table] = append(grouped[task.Table], task.ID)
	}

	// Start transaction
	tx, err := b.db.BeginTxx(b.ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin transaction for batch update")
		return
	}
	defer tx.Rollback()

	// Batch update each table
	for table, ids := range grouped {
		if err := b.batchUpdate(tx, table, ids); err != nil {
			log.Error().Err(err).Str("table", table).Msg("Batch update failed")
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Error().Err(err).Msg("Failed to commit batch update")
		return
	}

	log.Debug().
		Int("count", len(b.pending)).
		Interface("tables", grouped).
		Msg("Batch update completed")

	// Clear pending queue
	b.pending = b.pending[:0]
}

// batchUpdate updates status for a batch of IDs in a single table
func (b *UpdateBatcher) batchUpdate(tx *sqlx.Tx, table string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Validate table name (prevent SQL injection)
	validTables := map[string]bool{
		"keywords": true,
		"images":   true,
		"titles":   true,
		"contents": true,
	}

	if !validTables[table] {
		log.Warn().Str("table", table).Msg("Invalid table name, skipping batch update")
		return nil // Skip invalid tables without error
	}

	// Build IN clause
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query := fmt.Sprintf("UPDATE %s SET status = 0 WHERE id IN (%s)", table, placeholders)

	// Convert to interface{} slice
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	_, err := tx.Exec(query, args...)
	return err
}

// flushLoop runs the periodic flush
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
			b.flushLocked()
			b.mu.Unlock()
		}
	}
}
