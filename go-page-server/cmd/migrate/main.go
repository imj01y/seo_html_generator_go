// Package main provides a database migration command-line tool
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// Migration represents a database migration file
type Migration struct {
	Version  string
	Name     string
	FilePath string
	UpSQL    string
	DownSQL  string
}

// ExecutedMigration represents a migration record from the database
type ExecutedMigration struct {
	Version    string    `db:"version"`
	ExecutedAt time.Time `db:"executed_at"`
}

func main() {
	// Parse command line arguments
	dsn := flag.String("dsn", "", "Database connection string (required)")
	dir := flag.String("dir", "up", "Migration direction: up, down, status")
	target := flag.String("target", "", "Target migration version (optional)")
	migrationsPath := flag.String("path", "", "Path to migrations directory (default: ./migrations)")
	flag.Parse()

	// Validate required arguments
	if *dsn == "" {
		fmt.Println("Error: -dsn is required")
		fmt.Println("Usage: migrate -dsn <connection_string> [-dir up|down|status] [-target version]")
		os.Exit(1)
	}

	// Connect to database
	db, err := sqlx.Connect("mysql", *dsn)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Ensure migration tracking table exists
	if err := ensureMigrationTable(db); err != nil {
		fmt.Printf("Error creating migration table: %v\n", err)
		os.Exit(1)
	}

	// Determine migrations directory path
	migrationDir := *migrationsPath
	if migrationDir == "" {
		// Default to ./migrations relative to executable or current dir
		execPath, err := os.Executable()
		if err == nil {
			migrationDir = filepath.Join(filepath.Dir(execPath), "migrations")
		}
		if _, err := os.Stat(migrationDir); os.IsNotExist(err) {
			// Try current working directory
			migrationDir = "migrations"
		}
	}

	// Get all migrations from files
	migrations, err := getMigrations(migrationDir)
	if err != nil {
		fmt.Printf("Error reading migrations: %v\n", err)
		os.Exit(1)
	}

	// Get executed migrations from database
	executed, err := getExecutedMigrations(db)
	if err != nil {
		fmt.Printf("Error getting executed migrations: %v\n", err)
		os.Exit(1)
	}

	// Execute based on direction
	switch *dir {
	case "up":
		if err := migrateUp(db, migrations, executed, *target); err != nil {
			fmt.Printf("Migration error: %v\n", err)
			os.Exit(1)
		}
	case "down":
		if err := migrateDown(db, migrations, executed, *target); err != nil {
			fmt.Printf("Rollback error: %v\n", err)
			os.Exit(1)
		}
	case "status":
		showStatus(migrations, executed)
	default:
		fmt.Printf("Unknown direction: %s (use up, down, or status)\n", *dir)
		os.Exit(1)
	}
}

// ensureMigrationTable creates the schema_migrations table if it doesn't exist
func ensureMigrationTable(db *sqlx.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(50) PRIMARY KEY,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.Exec(query)
	return err
}

// getMigrations reads all migration files from the migrations directory
func getMigrations(dir string) ([]Migration, error) {
	var migrations []Migration

	// Read all SQL files in migrations directory
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("error listing migration files: %w", err)
	}

	// Regex to extract version and name from filename (e.g., 001_rename_baidu_token.sql)
	filePattern := regexp.MustCompile(`^(\d+)_(.+)\.sql$`)

	for _, file := range files {
		basename := filepath.Base(file)
		matches := filePattern.FindStringSubmatch(basename)
		if matches == nil {
			continue
		}

		version := matches[1]
		name := matches[2]

		// Read file content
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", file, err)
		}

		contentStr := string(content)

		// Extract UP and DOWN SQL
		upSQL := extractUpSQL(contentStr)
		downSQL := extractDownSQL(contentStr)

		// Skip empty migrations (like 000_init.sql which is just documentation)
		if strings.TrimSpace(upSQL) == "" && strings.TrimSpace(downSQL) == "" {
			continue
		}

		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			FilePath: file,
			UpSQL:    upSQL,
			DownSQL:  downSQL,
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// extractUpSQL extracts the UP migration SQL from file content
// It looks for content after "-- UP Migration" marker until "-- DOWN Migration" marker
func extractUpSQL(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inUpSection := false
	hasUpMarker := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for UP Migration marker
		if strings.HasPrefix(trimmed, "-- UP Migration") {
			inUpSection = true
			hasUpMarker = true
			continue
		}

		// Check for DOWN Migration marker
		if strings.HasPrefix(trimmed, "-- DOWN Migration") {
			inUpSection = false
			continue
		}

		// Collect UP section lines
		if inUpSection {
			// Skip comment-only lines
			if strings.HasPrefix(trimmed, "--") && !strings.Contains(trimmed, "Migration") {
				continue
			}
			if trimmed != "" {
				result = append(result, line)
			}
		}
	}

	// If no UP marker found, treat the whole content as UP SQL
	// (excluding comment lines and DOWN section)
	if !hasUpMarker {
		result = nil
		inDownSection := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Check for DOWN Migration marker
			if strings.HasPrefix(trimmed, "-- DOWN Migration") {
				inDownSection = true
				continue
			}

			// Skip comment lines and empty lines at the start
			if strings.HasPrefix(trimmed, "--") {
				continue
			}

			if !inDownSection && trimmed != "" {
				result = append(result, line)
			}
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

// extractDownSQL extracts the DOWN migration SQL from file content
// It looks for content after "-- DOWN Migration" marker
// Lines that start with "-- " (commented out SQL) have the comment prefix removed
func extractDownSQL(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inDownSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for DOWN Migration marker
		if strings.HasPrefix(trimmed, "-- DOWN Migration") {
			inDownSection = true
			continue
		}

		// Collect DOWN section lines
		if inDownSection {
			// Check if it's a commented SQL line (starts with "-- " followed by SQL)
			if strings.HasPrefix(trimmed, "-- ") {
				// Remove the comment prefix and add the SQL
				uncommented := strings.TrimPrefix(trimmed, "-- ")
				if uncommented != "" {
					result = append(result, uncommented)
				}
			} else if !strings.HasPrefix(trimmed, "--") && trimmed != "" {
				// Regular SQL line (not a pure comment)
				result = append(result, line)
			}
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

// getExecutedMigrations retrieves all executed migrations from the database
func getExecutedMigrations(db *sqlx.DB) (map[string]ExecutedMigration, error) {
	var records []ExecutedMigration
	err := db.Select(&records, "SELECT version, executed_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}

	result := make(map[string]ExecutedMigration)
	for _, r := range records {
		result[r.Version] = r
	}
	return result, nil
}

// migrateUp executes pending migrations
func migrateUp(db *sqlx.DB, migrations []Migration, executed map[string]ExecutedMigration, target string) error {
	pending := make([]Migration, 0)

	// Find pending migrations
	for _, m := range migrations {
		if _, ok := executed[m.Version]; !ok {
			pending = append(pending, m)
		}
	}

	if len(pending) == 0 {
		fmt.Println("No pending migrations")
		return nil
	}

	fmt.Printf("Found %d pending migration(s)\n", len(pending))

	// Execute each pending migration
	for _, m := range pending {
		// Check target version
		if target != "" && m.Version > target {
			fmt.Printf("Reached target version %s, stopping\n", target)
			break
		}

		if m.UpSQL == "" {
			fmt.Printf("Skipping %s_%s (no UP SQL)\n", m.Version, m.Name)
			continue
		}

		fmt.Printf("Migrating %s_%s...\n", m.Version, m.Name)

		// Execute in transaction
		tx, err := db.Beginx()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Execute migration SQL (may contain multiple statements)
		statements := splitStatements(m.UpSQL)
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to execute %s_%s: %w\nSQL: %s", m.Version, m.Name, err, stmt)
			}
		}

		// Record migration
		_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.Version)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		fmt.Printf("  Done: %s_%s\n", m.Version, m.Name)
	}

	fmt.Println("Migration completed")
	return nil
}

// migrateDown rolls back executed migrations
func migrateDown(db *sqlx.DB, migrations []Migration, executed map[string]ExecutedMigration, target string) error {
	// Get executed migrations in reverse order
	executedList := make([]Migration, 0)
	for _, m := range migrations {
		if _, ok := executed[m.Version]; ok {
			executedList = append(executedList, m)
		}
	}

	if len(executedList) == 0 {
		fmt.Println("No migrations to rollback")
		return nil
	}

	// Reverse order for rollback
	sort.Slice(executedList, func(i, j int) bool {
		return executedList[i].Version > executedList[j].Version
	})

	fmt.Printf("Found %d executed migration(s)\n", len(executedList))

	// Rollback migrations
	for _, m := range executedList {
		// Check target version (stop before reaching target)
		if target != "" && m.Version <= target {
			fmt.Printf("Reached target version %s, stopping\n", target)
			break
		}

		if m.DownSQL == "" {
			fmt.Printf("Warning: %s_%s has no DOWN SQL, skipping\n", m.Version, m.Name)
			continue
		}

		fmt.Printf("Rolling back %s_%s...\n", m.Version, m.Name)

		// Execute in transaction
		tx, err := db.Beginx()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Execute rollback SQL (may contain multiple statements)
		statements := splitStatements(m.DownSQL)
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to rollback %s_%s: %w\nSQL: %s", m.Version, m.Name, err, stmt)
			}
		}

		// Remove migration record
		_, err = tx.Exec("DELETE FROM schema_migrations WHERE version = ?", m.Version)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to remove migration record %s: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		fmt.Printf("  Rolled back: %s_%s\n", m.Version, m.Name)

		// If no target specified, only rollback one migration
		if target == "" {
			break
		}
	}

	fmt.Println("Rollback completed")
	return nil
}

// showStatus displays the current migration status
func showStatus(migrations []Migration, executed map[string]ExecutedMigration) {
	fmt.Println("Migration Status")
	fmt.Println("================")
	fmt.Printf("%-10s %-40s %-10s %s\n", "Version", "Name", "Status", "Executed At")
	fmt.Println(strings.Repeat("-", 80))

	for _, m := range migrations {
		status := "Pending"
		executedAt := ""
		if e, ok := executed[m.Version]; ok {
			status = "Done"
			executedAt = e.ExecutedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("%-10s %-40s %-10s %s\n", m.Version, truncate(m.Name, 40), status, executedAt)
	}

	fmt.Println(strings.Repeat("-", 80))

	// Summary
	pending := 0
	done := 0
	for _, m := range migrations {
		if _, ok := executed[m.Version]; ok {
			done++
		} else {
			pending++
		}
	}
	fmt.Printf("Total: %d migrations (%d done, %d pending)\n", len(migrations), done, pending)
}

// splitStatements splits SQL content into individual statements
func splitStatements(sql string) []string {
	// Simple split by semicolon
	// Note: This doesn't handle semicolons inside strings or stored procedures
	// For more complex cases, a proper SQL parser would be needed
	statements := strings.Split(sql, ";")
	result := make([]string, 0, len(statements))
	for _, s := range statements {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
