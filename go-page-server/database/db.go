// Package database handles MySQL database connections
package database

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/config"
)

var db *sqlx.DB

// Init initializes the database connection pool
func Init(cfg *config.DatabaseConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)

	var err error
	db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool for high concurrency (500 concurrent requests)
	// Use at least 50 connections, or config value if higher
	maxConns := cfg.PoolSize
	if maxConns < 50 {
		maxConns = 50
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns) // Keep all connections alive to avoid reconnection overhead
	db.SetConnMaxLifetime(30 * time.Minute) // Shorter lifetime to avoid stale connections

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("database", cfg.Database).
		Int("pool_size", cfg.PoolSize).
		Msg("Database connection established")

	return nil
}

// GetDB returns the database connection
func GetDB() *sqlx.DB {
	return db
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// FetchOne fetches a single row
func FetchOne(dest interface{}, query string, args ...interface{}) error {
	return db.Get(dest, query, args...)
}

// FetchAll fetches multiple rows
func FetchAll(dest interface{}, query string, args ...interface{}) error {
	return db.Select(dest, query, args...)
}

// Execute executes a query without returning results
func Execute(query string, args ...interface{}) error {
	_, err := db.Exec(query, args...)
	return err
}

// Insert inserts a record and returns the last insert ID
func Insert(table string, data map[string]interface{}) (int64, error) {
	columns := ""
	placeholders := ""
	values := make([]interface{}, 0, len(data))

	for col, val := range data {
		if columns != "" {
			columns += ", "
			placeholders += ", "
		}
		columns += col
		placeholders += "?"
		values = append(values, val)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, columns, placeholders)
	result, err := db.Exec(query, values...)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}
