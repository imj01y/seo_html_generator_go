// Package main provides tests for the database migration tool
package main

import (
	"testing"
)

// TestExtractUpSQL tests extracting UP SQL from migration content
func TestExtractUpSQL(t *testing.T) {
	content := `-- UP Migration
ALTER TABLE sites ADD COLUMN new_field VARCHAR(255);
ALTER TABLE sites ADD INDEX idx_new_field (new_field);

-- DOWN Migration
-- ALTER TABLE sites DROP INDEX idx_new_field;
-- ALTER TABLE sites DROP COLUMN new_field;
`

	result := extractUpSQL(content)

	// Verify UP SQL is extracted
	if result == "" {
		t.Error("Expected UP SQL to be extracted, got empty string")
	}

	// Verify it contains the UP statements
	if !contains(result, "ALTER TABLE sites ADD COLUMN new_field") {
		t.Errorf("Expected UP SQL to contain 'ALTER TABLE sites ADD COLUMN new_field', got: %s", result)
	}

	if !contains(result, "ADD INDEX idx_new_field") {
		t.Errorf("Expected UP SQL to contain 'ADD INDEX idx_new_field', got: %s", result)
	}

	// Verify it does NOT contain DOWN statements
	if contains(result, "DROP INDEX") {
		t.Errorf("UP SQL should not contain DOWN statements (DROP INDEX), got: %s", result)
	}

	if contains(result, "DROP COLUMN") {
		t.Errorf("UP SQL should not contain DOWN statements (DROP COLUMN), got: %s", result)
	}
}

// TestExtractDownSQL tests extracting DOWN SQL from migration content
func TestExtractDownSQL(t *testing.T) {
	content := `-- UP Migration
ALTER TABLE sites ADD COLUMN new_field VARCHAR(255);

-- DOWN Migration
-- ALTER TABLE sites DROP COLUMN new_field;
`

	result := extractDownSQL(content)

	// Verify DOWN SQL is extracted
	if result == "" {
		t.Error("Expected DOWN SQL to be extracted, got empty string")
	}

	// Verify comment prefix is removed
	if contains(result, "-- ALTER") {
		t.Errorf("DOWN SQL should have comment prefix removed, got: %s", result)
	}

	// Verify the SQL statement is present
	if !contains(result, "ALTER TABLE sites DROP COLUMN new_field") {
		t.Errorf("Expected DOWN SQL to contain 'ALTER TABLE sites DROP COLUMN new_field', got: %s", result)
	}
}

// TestExtractDownSQL_NoDown tests extracting DOWN SQL when no DOWN section exists
func TestExtractDownSQL_NoDown(t *testing.T) {
	content := `-- UP Migration
ALTER TABLE sites ADD COLUMN new_field VARCHAR(255);
ALTER TABLE sites ADD INDEX idx_new_field (new_field);
`

	result := extractDownSQL(content)

	// Verify empty string is returned when no DOWN section
	if result != "" {
		t.Errorf("Expected empty string when no DOWN section, got: %s", result)
	}
}

// TestExtractUpSQL_NoMarker tests extracting UP SQL when there's no explicit UP marker
func TestExtractUpSQL_NoMarker(t *testing.T) {
	content := `ALTER TABLE sites ADD COLUMN new_field VARCHAR(255);
ALTER TABLE sites ADD INDEX idx_new_field (new_field);

-- DOWN Migration
-- ALTER TABLE sites DROP COLUMN new_field;
`

	result := extractUpSQL(content)

	// Verify UP SQL is extracted (whole content before DOWN marker)
	if result == "" {
		t.Error("Expected UP SQL to be extracted, got empty string")
	}

	// Verify it contains the SQL statements
	if !contains(result, "ALTER TABLE sites ADD COLUMN new_field") {
		t.Errorf("Expected UP SQL to contain 'ALTER TABLE sites ADD COLUMN new_field', got: %s", result)
	}

	// Verify it does NOT contain DOWN statements
	if contains(result, "DROP COLUMN") {
		t.Errorf("UP SQL should not contain DOWN statements, got: %s", result)
	}
}

// TestExtractDownSQL_MultipleStatements tests extracting multiple DOWN SQL statements
func TestExtractDownSQL_MultipleStatements(t *testing.T) {
	content := `-- UP Migration
ALTER TABLE sites ADD COLUMN field1 VARCHAR(255);
ALTER TABLE sites ADD COLUMN field2 VARCHAR(255);

-- DOWN Migration
-- ALTER TABLE sites DROP COLUMN field2;
-- ALTER TABLE sites DROP COLUMN field1;
`

	result := extractDownSQL(content)

	// Verify both statements are extracted
	if !contains(result, "DROP COLUMN field1") {
		t.Errorf("Expected DOWN SQL to contain 'DROP COLUMN field1', got: %s", result)
	}

	if !contains(result, "DROP COLUMN field2") {
		t.Errorf("Expected DOWN SQL to contain 'DROP COLUMN field2', got: %s", result)
	}
}

// contains checks if str contains substr
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(str) > 0 && containsHelper(str, substr))
}

func containsHelper(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
