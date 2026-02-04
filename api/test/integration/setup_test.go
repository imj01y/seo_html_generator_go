//go:build integration

// Package integration provides integration tests that require a real database.
// These tests are excluded from normal test runs and must be explicitly enabled
// with the -tags=integration flag.
//
// Usage:
//
//	go test -tags=integration ./test/integration/... -v
//
// Prerequisites:
//   - A running MySQL database with the seo_generator schema
//   - Environment variables or config.yaml with database credentials
package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/di"
	"seo-generator/api/pkg/config"
)

var (
	testContainer *di.Container
	testDB        *sqlx.DB
	testConfig    *config.Config
)

// TestMain sets up the integration test environment
func TestMain(m *testing.M) {
	// Configure logger for tests
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
		With().Timestamp().Logger()

	// Find project root
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		log.Fatal().Msg("Could not find project root (config.yaml not found)")
	}

	// Load configuration
	configPath := filepath.Join(projectRoot, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", configPath).Msg("Failed to load config")
	}
	testConfig = cfg

	// Connect to database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.Charset,
	)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	testDB = db

	// Create container
	testContainer = di.NewContainer(db, cfg)

	log.Info().
		Str("host", cfg.Database.Host).
		Str("database", cfg.Database.Database).
		Msg("Integration test environment initialized")

	// Run tests
	code := m.Run()

	// Cleanup
	testContainer.Close()
	testDB.Close()

	os.Exit(code)
}

// findProjectRoot locates the project root directory
func findProjectRoot() string {
	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// getTestContext returns a context for tests
func getTestContext() context.Context {
	return context.Background()
}
