package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Local is a Store backed by a folder. Keys use "/" like S3, and map to
// paths under the folder.
type Local struct {
	dir string
}

func NewLocal(dir string) *Local {
	return &Local{dir: dir}
}

func (l *Local) path(key string) (string, error) {
	if key == "" || strings.HasPrefix(key, "/") || slices.Contains(strings.Split(key, "/"), "..") {
		return "", fmt.Errorf("store: invalid key %q", key)
	}
	return filepath.Join(l.dir, filepath.FromSlash(key)), nil
}

func (l *Local) Put(ctx context.Context, key string, body []byte) error {
	path, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	// Write to a temporary file in the same folder, then rename it into
	// place, so the file only ever appears complete.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("store: writing %s: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: writing %s: %w", key, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("store: writing %s: %w", key, err)
	}
	return nil
}

func (l *Local) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	err := filepath.WalkDir(l.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // an empty store has no folder yet
			}
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		rel, err := filepath.Rel(l.dir, path)
		if err != nil {
			return err
		}
		if key := filepath.ToSlash(rel); strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("store: listing %q: %w", prefix, err)
	}
	slices.Sort(keys)
	return keys, nil
}

func (l *Local) Get(ctx context.Context, key string) ([]byte, error) {
	path, err := l.path(key)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("store: reading %s: %w", key, err)
	}
	return body, nil
}
