package storage

import (
	"errors"
	"fmt"
)

// Common errors.
var (
	ErrNotFound       = errors.New("storage: file not found")
	ErrAlreadyExists  = errors.New("storage: file already exists")
	ErrPermission     = errors.New("storage: permission denied")
	ErrInvalidKey     = errors.New("storage: invalid key")
	ErrNotImplemented = errors.New("storage: not implemented")
	ErrClosed         = errors.New("storage: storage is closed")
)

// Error represents a storage error with additional context.
type Error struct {
	Op      string // Operation that failed (e.g., "upload", "download")
	Driver  string // Driver name (e.g., "aliyun", "s3")
	Key     string // File key
	Err     error  // Underlying error
}

func (e *Error) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("storage: %s %s [%s]: %v", e.Driver, e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("storage: %s %s: %v", e.Driver, e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// NewError creates a new storage error.
func NewError(driver, op, key string, err error) *Error {
	return &Error{
		Op:     op,
		Driver: driver,
		Key:    key,
		Err:    err,
	}
}

// IsNotFound checks if the error is a "not found" error.
func IsNotFoundError(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	return IsNotExist(err)
}

// IsPermissionError checks if the error is a permission error.
func IsPermissionError(err error) bool {
	return errors.Is(err, ErrPermission)
}
