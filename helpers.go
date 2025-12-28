package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UploadFile is a convenience function to upload a file from disk.
func UploadFile(ctx context.Context, s Storage, key, filePath string, opts ...UploadOption) (*UploadResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to open file: %w", err)
	}
	defer f.Close()

	// Auto-detect content type if not specified
	hasContentType := false
	for _, opt := range opts {
		testOpts := &UploadOptions{}
		opt(testOpts)
		if testOpts.ContentType != "" {
			hasContentType = true
			break
		}
	}

	if !hasContentType {
		ct := DetectContentType(filePath)
		if ct != "" {
			opts = append(opts, WithContentType(ct))
		}
	}

	return s.Upload(ctx, key, f, opts...)
}

// DownloadToFile is a convenience function to download a file to disk.
func DownloadToFile(ctx context.Context, s Storage, key, filePath string) error {
	reader, err := s.Download(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("storage: failed to create directory: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("storage: failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("storage: failed to write file: %w", err)
	}

	return nil
}

// DetectContentType detects the content type based on file extension.
func DetectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return "application/octet-stream"
	}

	// Common types that mime package might not have
	commonTypes := map[string]string{
		".md":   "text/markdown",
		".yaml": "text/yaml",
		".yml":  "text/yaml",
		".ts":   "text/typescript",
		".tsx":  "text/typescript",
		".vue":  "text/x-vue",
		".go":   "text/x-go",
		".rs":   "text/x-rust",
		".webp": "image/webp",
		".avif": "image/avif",
		".heic": "image/heic",
		".heif": "image/heif",
		".woff": "font/woff",
		".woff2": "font/woff2",
	}

	if ct, ok := commonTypes[ext]; ok {
		return ct
	}

	ct := mime.TypeByExtension(ext)
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}

// GenerateKey generates a unique key for a file.
// Format: prefix/2006/01/02/uuid.ext
func GenerateKey(prefix, filename string) string {
	ext := filepath.Ext(filename)
	now := time.Now()
	uuid := generateUUID()

	parts := []string{}
	if prefix != "" {
		parts = append(parts, strings.Trim(prefix, "/"))
	}
	parts = append(parts, now.Format("2006/01/02"))
	parts = append(parts, uuid+ext)

	return strings.Join(parts, "/")
}

// GenerateKeyFlat generates a unique key without date directories.
// Format: prefix/uuid.ext
func GenerateKeyFlat(prefix, filename string) string {
	ext := filepath.Ext(filename)
	uuid := generateUUID()

	if prefix == "" {
		return uuid + ext
	}
	return strings.Trim(prefix, "/") + "/" + uuid + ext
}

// generateUUID generates a simple UUID-like string.
func generateUUID() string {
	// Simple implementation using timestamp + random
	now := time.Now().UnixNano()
	return fmt.Sprintf("%x", now)
}

// Must panics if err is not nil. Useful for initialization.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// IsNotExist checks if the error indicates the file does not exist.
func IsNotExist(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "not found") ||
		strings.Contains(s, "not exist") ||
		strings.Contains(s, "NoSuchKey") ||
		strings.Contains(s, "404")
}

// Retry retries a function with exponential backoff.
func Retry(ctx context.Context, maxAttempts int, fn func() error) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			// Exponential backoff: 100ms, 200ms, 400ms, ...
			delay := time.Duration(100<<i) * time.Millisecond
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}
		return nil
	}
	return fmt.Errorf("storage: max retries exceeded: %w", lastErr)
}

// ProgressReader wraps a reader to track progress.
type ProgressReader struct {
	reader   io.Reader
	total    int64
	uploaded int64
	fn       func(uploaded, total int64)
}

// NewProgressReader creates a new progress reader.
func NewProgressReader(r io.Reader, total int64, fn func(uploaded, total int64)) *ProgressReader {
	return &ProgressReader{
		reader: r,
		total:  total,
		fn:     fn,
	}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.uploaded += int64(n)
		if pr.fn != nil {
			pr.fn(pr.uploaded, pr.total)
		}
	}
	return n, err
}

// SizeReader wraps a reader to get its size.
type SizeReader struct {
	io.Reader
	size int64
}

// NewSizeReader creates a reader that knows its size.
func NewSizeReader(r io.Reader, size int64) *SizeReader {
	return &SizeReader{Reader: r, size: size}
}

// Size returns the total size.
func (sr *SizeReader) Size() int64 {
	return sr.size
}
