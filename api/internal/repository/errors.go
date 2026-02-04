// Package repository provides data access layer interfaces and implementations.
// This file defines common error types used across all repository operations.
package repository

import (
	"errors"
	"strings"
)

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

// IsDuplicateKeyError 检测是否为重复键错误
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// MySQL: Error 1062 - Duplicate entry
	return strings.Contains(errMsg, "Duplicate entry") ||
		strings.Contains(errMsg, "duplicate key")
}
