// Package cache defines a small key-value cache interface.
package cache

import (
	"context"
	"errors"
)

var (
	ErrNilContext = errors.New("cache context is required")
	ErrInvalidKey = errors.New("invalid cache key")
	ErrNotFound   = errors.New("cache key not found")
	ErrTooLarge   = errors.New("cache entry exceeds byte capacity")
	ErrClosed     = errors.New("cache is closed")
)

// Store operations require a non-nil context; use context.Background when no deadline is needed.
type Store interface {
	Set(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}
