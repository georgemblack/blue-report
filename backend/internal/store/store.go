// Package store saves and reads whole files by key, like an S3 bucket. In
// production it's backed by S3; locally and in tests it's a folder on disk.
package store

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Get when no file has the given key.
var ErrNotFound = errors.New("store: not found")

type Store interface {
	// Put saves a whole file. It either fully succeeds or leaves nothing
	// behind, so a crash can't leave half a file.
	Put(ctx context.Context, key string, body []byte) error

	// List returns every key that starts with prefix, in sorted order.
	List(ctx context.Context, prefix string) ([]string, error)

	// Get returns a whole file, or ErrNotFound.
	Get(ctx context.Context, key string) ([]byte, error)
}
