// Package local provides a local filesystem storage driver.
package local

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("local", New)
}

// Config for local storage.
type Config struct {
	Root    string // Root directory for file storage
	BaseURL string // Base URL for generating public URLs (optional)
	Perm    os.FileMode
}

// Local implements storage.Storage for local filesystem.
type Local struct {
	root    string
	baseURL string
	perm    os.FileMode
}

// Ensure Local implements AdvancedStorage.
var _ storage.AdvancedStorage = (*Local)(nil)

// New creates a new local storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	root, _ := cfg["root"].(string)
	if root == "" {
		return nil, fmt.Errorf("local: root path is required")
	}

	// Expand ~ to home directory
	if root[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("local: failed to get home directory: %w", err)
		}
		root = filepath.Join(home, root[1:])
	}

	// Ensure root directory exists
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("local: failed to create root directory: %w", err)
	}

	baseURL, _ := cfg["base_url"].(string)

	perm := os.FileMode(0644)
	if p, ok := cfg["perm"].(int); ok {
		perm = os.FileMode(p)
	}

	return &Local{
		root:    root,
		baseURL: baseURL,
		perm:    perm,
	}, nil
}

func (l *Local) fullPath(key string) string {
	return filepath.Join(l.root, filepath.Clean(key))
}

// Upload uploads a file to local filesystem.
func (l *Local) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	path := l.fullPath(key)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("local: failed to create directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, l.perm)
	if err != nil {
		return nil, fmt.Errorf("local: failed to create file: %w", err)
	}
	defer f.Close()

	size, err := io.Copy(f, reader)
	if err != nil {
		return nil, fmt.Errorf("local: failed to write file: %w", err)
	}

	result := &storage.UploadResult{
		Key:  key,
		Size: size,
	}

	if l.baseURL != "" {
		result.URL = l.baseURL + "/" + url.PathEscape(key)
	}

	return result, nil
}

// Download downloads a file from local filesystem.
func (l *Local) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	path := l.fullPath(key)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("local: file not found: %s", key)
		}
		return nil, fmt.Errorf("local: failed to open file: %w", err)
	}
	return f, nil
}

// Delete deletes a file from local filesystem.
func (l *Local) Delete(ctx context.Context, key string) error {
	path := l.fullPath(key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, not an error
		}
		return fmt.Errorf("local: failed to delete file: %w", err)
	}
	return nil
}

// Exists checks if a file exists.
func (l *Local) Exists(ctx context.Context, key string) (bool, error) {
	path := l.fullPath(key)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("local: failed to check file: %w", err)
}

// URL returns the public URL of a file.
func (l *Local) URL(ctx context.Context, key string) (string, error) {
	if l.baseURL == "" {
		return "", fmt.Errorf("local: base_url not configured")
	}
	return l.baseURL + "/" + url.PathEscape(key), nil
}

// Close is a no-op for local storage.
func (l *Local) Close() error {
	return nil
}

// --- AdvancedStorage implementation ---

// SignedURL is not supported for local storage.
func (l *Local) SignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	// Local storage doesn't support signed URLs, just return the regular URL
	return l.URL(ctx, key)
}

// List lists files with the given prefix.
func (l *Local) List(ctx context.Context, prefix string, opts ...storage.ListOption) (*storage.ListResult, error) {
	options := &storage.ListOptions{MaxKeys: 1000}
	for _, opt := range opts {
		opt(options)
	}

	searchPath := l.fullPath(prefix)
	var files []storage.FileInfo

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If the prefix path doesn't exist, return empty result
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative key
		relPath, _ := filepath.Rel(l.root, path)
		key := filepath.ToSlash(relPath)

		files = append(files, storage.FileInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})

		if len(files) >= options.MaxKeys {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("local: list failed: %w", err)
	}

	return &storage.ListResult{
		Files:       files,
		IsTruncated: len(files) >= options.MaxKeys,
	}, nil
}

// Copy copies a file from src to dst.
func (l *Local) Copy(ctx context.Context, src, dst string) error {
	srcPath := l.fullPath(src)
	dstPath := l.fullPath(dst)

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("local: failed to create directory: %w", err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("local: failed to open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("local: failed to create destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("local: copy failed: %w", err)
	}

	return nil
}

// Move moves a file from src to dst.
func (l *Local) Move(ctx context.Context, src, dst string) error {
	srcPath := l.fullPath(src)
	dstPath := l.fullPath(dst)

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("local: failed to create directory: %w", err)
	}

	if err := os.Rename(srcPath, dstPath); err != nil {
		return fmt.Errorf("local: move failed: %w", err)
	}

	return nil
}

// Size returns the size of a file.
func (l *Local) Size(ctx context.Context, key string) (int64, error) {
	path := l.fullPath(key)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, storage.ErrNotFound
		}
		return 0, fmt.Errorf("local: failed to get size: %w", err)
	}
	return info.Size(), nil
}

// Metadata returns the metadata of a file.
func (l *Local) Metadata(ctx context.Context, key string) (*storage.FileInfo, error) {
	path := l.fullPath(key)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("local: failed to get metadata: %w", err)
	}

	return &storage.FileInfo{
		Key:          key,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		ContentType:  storage.DetectContentType(key),
	}, nil
}
