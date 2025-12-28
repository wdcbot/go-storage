package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func init() {
	// Local driver is built-in, no need to import separately
	Register("local", newLocalStorage)
}

// localStorage implements Storage for local filesystem.
type localStorage struct {
	root    string
	baseURL string
	perm    os.FileMode
}

func newLocalStorage(cfg map[string]any) (Storage, error) {
	root, _ := cfg["root"].(string)
	if root == "" {
		return nil, fmt.Errorf("local: 'root' is required")
	}

	// Expand ~ to home directory
	if len(root) > 0 && root[0] == '~' {
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

	return &localStorage{
		root:    root,
		baseURL: baseURL,
		perm:    perm,
	}, nil
}

func (l *localStorage) fullPath(key string) string {
	return filepath.Join(l.root, filepath.Clean(key))
}

func (l *localStorage) Upload(ctx context.Context, key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error) {
	path := l.fullPath(key)

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

	result := &UploadResult{Key: key, Size: size}
	if l.baseURL != "" {
		result.URL = l.baseURL + "/" + url.PathEscape(key)
	}

	return result, nil
}

func (l *localStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	path := l.fullPath(key)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("local: failed to open file: %w", err)
	}
	return f, nil
}

func (l *localStorage) Delete(ctx context.Context, key string) error {
	path := l.fullPath(key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("local: failed to delete file: %w", err)
	}
	return nil
}

func (l *localStorage) Exists(ctx context.Context, key string) (bool, error) {
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

func (l *localStorage) URL(ctx context.Context, key string) (string, error) {
	if l.baseURL == "" {
		return "", fmt.Errorf("local: base_url not configured")
	}
	return l.baseURL + "/" + url.PathEscape(key), nil
}

func (l *localStorage) Close() error {
	return nil
}

// --- AdvancedStorage ---

func (l *localStorage) SignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return l.URL(ctx, key)
}

func (l *localStorage) List(ctx context.Context, prefix string, opts ...ListOption) (*ListResult, error) {
	options := &ListOptions{MaxKeys: 1000}
	for _, opt := range opts {
		opt(options)
	}

	searchPath := l.fullPath(prefix)
	var files []FileInfo

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(l.root, path)
		files = append(files, FileInfo{
			Key:          filepath.ToSlash(relPath),
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

	return &ListResult{
		Files:       files,
		IsTruncated: len(files) >= options.MaxKeys,
	}, nil
}

func (l *localStorage) Copy(ctx context.Context, src, dst string) error {
	srcPath := l.fullPath(src)
	dstPath := l.fullPath(dst)

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

func (l *localStorage) Move(ctx context.Context, src, dst string) error {
	srcPath := l.fullPath(src)
	dstPath := l.fullPath(dst)

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("local: failed to create directory: %w", err)
	}

	if err := os.Rename(srcPath, dstPath); err != nil {
		return fmt.Errorf("local: move failed: %w", err)
	}
	return nil
}

func (l *localStorage) Size(ctx context.Context, key string) (int64, error) {
	path := l.fullPath(key)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("local: failed to get size: %w", err)
	}
	return info.Size(), nil
}

func (l *localStorage) Metadata(ctx context.Context, key string) (*FileInfo, error) {
	path := l.fullPath(key)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("local: failed to get metadata: %w", err)
	}

	return &FileInfo{
		Key:          key,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		ContentType:  DetectContentType(key),
	}, nil
}

// Ensure localStorage implements AdvancedStorage
var _ AdvancedStorage = (*localStorage)(nil)
