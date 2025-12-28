// Package storage provides a pluggable file storage solution for Go developers.
// Configure once via YAML/JSON, use everywhere without writing initialization code.
package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Storage is the main interface that all storage backends must implement.
type Storage interface {
	// Upload uploads a file to the storage backend.
	Upload(ctx context.Context, key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error)

	// Download downloads a file from the storage backend.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete deletes a file from the storage backend.
	Delete(ctx context.Context, key string) error

	// Exists checks if a file exists in the storage backend.
	Exists(ctx context.Context, key string) (bool, error)

	// URL returns the public URL of a file (if supported).
	URL(ctx context.Context, key string) (string, error)

	// Close releases any resources held by the storage backend.
	Close() error
}

// AdvancedStorage extends Storage with optional advanced features.
// Not all drivers support these methods.
type AdvancedStorage interface {
	Storage

	// SignedURL generates a pre-signed URL for temporary access to private files.
	// expires specifies how long the URL should be valid.
	SignedURL(ctx context.Context, key string, expires time.Duration) (string, error)

	// List lists files in the storage with the given prefix.
	List(ctx context.Context, prefix string, opts ...ListOption) (*ListResult, error)

	// Copy copies a file from src to dst within the same storage.
	Copy(ctx context.Context, src, dst string) error

	// Move moves a file from src to dst within the same storage.
	Move(ctx context.Context, src, dst string) error

	// Size returns the size of a file in bytes.
	Size(ctx context.Context, key string) (int64, error)

	// Metadata returns the metadata of a file.
	Metadata(ctx context.Context, key string) (*FileInfo, error)
}

// FileInfo contains metadata about a file.
type FileInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
	Metadata     map[string]string
}

// ListResult contains the result of a List operation.
type ListResult struct {
	Files       []FileInfo
	NextMarker  string // For pagination
	IsTruncated bool   // Whether there are more results
}

// ListOptions configures list behavior.
type ListOptions struct {
	MaxKeys   int
	Marker    string // Start listing after this key
	Delimiter string // e.g., "/" for directory-like listing
}

// ListOption is a functional option for List.
type ListOption func(*ListOptions)

// WithMaxKeys sets the maximum number of keys to return.
func WithMaxKeys(n int) ListOption {
	return func(o *ListOptions) {
		o.MaxKeys = n
	}
}

// WithMarker sets the marker for pagination.
func WithMarker(marker string) ListOption {
	return func(o *ListOptions) {
		o.Marker = marker
	}
}

// WithDelimiter sets the delimiter for directory-like listing.
func WithDelimiter(d string) ListOption {
	return func(o *ListOptions) {
		o.Delimiter = d
	}
}

// UploadResult contains information about an uploaded file.
type UploadResult struct {
	Key      string            // The key/path of the uploaded file
	URL      string            // Public URL (if available)
	Size     int64             // Size in bytes
	ETag     string            // ETag/checksum (if available)
	Metadata map[string]string // Additional metadata
}

// UploadOptions configures upload behavior.
type UploadOptions struct {
	ContentType        string
	ContentDisposition string
	Metadata           map[string]string
	ACL                string // e.g., "public-read", "private"
	ProgressFn         func(uploaded, total int64) // Progress callback
}

// UploadOption is a functional option for Upload.
type UploadOption func(*UploadOptions)

// WithContentType sets the content type.
func WithContentType(ct string) UploadOption {
	return func(o *UploadOptions) {
		o.ContentType = ct
	}
}

// WithContentDisposition sets the content disposition.
func WithContentDisposition(cd string) UploadOption {
	return func(o *UploadOptions) {
		o.ContentDisposition = cd
	}
}

// WithMetadata sets custom metadata.
func WithMetadata(m map[string]string) UploadOption {
	return func(o *UploadOptions) {
		o.Metadata = m
	}
}

// WithACL sets the access control.
func WithACL(acl string) UploadOption {
	return func(o *UploadOptions) {
		o.ACL = acl
	}
}

// WithProgress sets a progress callback for upload.
func WithProgress(fn func(uploaded, total int64)) UploadOption {
	return func(o *UploadOptions) {
		o.ProgressFn = fn
	}
}

// Driver is a factory function that creates a Storage instance from config.
type Driver func(cfg map[string]any) (Storage, error)

var (
	drivers   = make(map[string]Driver)
	driversMu sync.RWMutex
)

// Register registers a storage driver.
// This is typically called in init() by each driver package.
func Register(name string, driver Driver) {
	driversMu.Lock()
	defer driversMu.Unlock()
	if driver == nil {
		panic("storage: Register driver is nil")
	}
	if _, exists := drivers[name]; exists {
		panic("storage: Register called twice for driver " + name)
	}
	drivers[name] = driver
}

// Drivers returns a list of registered driver names.
func Drivers() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	return names
}

// Open creates a Storage instance using the specified driver and config.
func Open(driverName string, cfg map[string]any) (Storage, error) {
	driversMu.RLock()
	driver, ok := drivers[driverName]
	driversMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("storage: unknown driver %q (forgotten import?)", driverName)
	}
	return driver(cfg)
}
