package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage persists files on disk under a base directory.
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage ensures the base directory exists and returns a handle.
func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if baseDir == "" {
		baseDir = "./exports"
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create exports directory: %w", err)
	}
	return &LocalStorage{baseDir: baseDir}, nil
}

// Save writes the given bytes to the provided relative path under the base dir.
func (s *LocalStorage) Save(filename string, data []byte) (string, error) {
	path := s.resolve(filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("prepare export directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return filename, nil
}

// SaveStream copies from reader into the target file path.
func (s *LocalStorage) SaveStream(filename string, r io.Reader) (string, error) {
	path := s.resolve(filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("prepare export directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create export file: %w", err)
	}
	defer file.Close() //nolint:errcheck
	if _, err := io.Copy(file, r); err != nil {
		return "", fmt.Errorf("write export stream: %w", err)
	}
	return filename, nil
}

// Open returns a read-only handle for the stored file.
func (s *LocalStorage) Open(filename string) (*os.File, error) {
	path := s.resolve(filename)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open export file: %w", err)
	}
	return file, nil
}

// Delete removes a stored file if present.
func (s *LocalStorage) Delete(filename string) error {
	path := s.resolve(filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete export file: %w", err)
	}
	return nil
}

// CleanupOlderThan removes files older than the provided TTL and returns deleted names.
func (s *LocalStorage) CleanupOlderThan(ttl time.Duration) ([]string, error) {
	cutoff := time.Now().Add(-ttl)
	deleted := make([]string, 0)
	err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		rel, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			rel = path
		}
		deleted = append(deleted, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cleanup exports: %w", err)
	}
	return deleted, nil
}

// Path exposes the underlying absolute path (useful for debugging).
func (s *LocalStorage) Path(filename string) string {
	return s.resolve(filename)
}

func (s *LocalStorage) resolve(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(s.baseDir, filename)
}
