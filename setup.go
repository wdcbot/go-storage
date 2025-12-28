package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// global default manager
var (
	defaultMgr *Manager
	defaultMu  sync.RWMutex
)

// Setup initializes storage from a map (works with viper, koanf, etc).
//
// Example with viper:
//
//	viper.ReadInConfig()
//	storage.Setup(viper.GetStringMap("storage"))
//
// Example with raw map:
//
//	storage.Setup(map[string]any{
//	    "default": "local",
//	    "disks": map[string]any{
//	        "local": map[string]any{
//	            "driver": "local",
//	            "root":   "./uploads",
//	        },
//	    },
//	})
func Setup(cfg map[string]any) error {
	c, err := parseConfigMap(cfg)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	defaultMgr = NewManager(c)
	defaultMu.Unlock()
	return nil
}

// MustSetup is like Setup but panics on error.
func MustSetup(cfg map[string]any) {
	if err := Setup(cfg); err != nil {
		panic(err)
	}
}

// parseConfigMap converts a map to Config.
// Supports both "storages" and "disks" as key names.
func parseConfigMap(m map[string]any) (*Config, error) {
	cfg := &Config{
		Storages: make(map[string]StorageConfig),
	}

	// Get default
	if d, ok := m["default"].(string); ok {
		cfg.Default = d
	}

	// Get disks/storages
	var disksRaw map[string]any
	if d, ok := m["disks"].(map[string]any); ok {
		disksRaw = d
	} else if d, ok := m["storages"].(map[string]any); ok {
		disksRaw = d
	}

	if disksRaw == nil {
		return nil, fmt.Errorf("storage: no 'disks' or 'storages' found in config")
	}

	for name, diskRaw := range disksRaw {
		disk, ok := diskRaw.(map[string]any)
		if !ok {
			continue
		}

		sc := StorageConfig{
			Options: make(map[string]any),
		}

		// Get driver
		if d, ok := disk["driver"].(string); ok {
			sc.Driver = d
		} else {
			return nil, fmt.Errorf("storage: disk %q missing 'driver'", name)
		}

		// All other fields go to options
		for k, v := range disk {
			if k != "driver" {
				sc.Options[k] = v
			}
		}

		cfg.Storages[name] = sc
	}

	return cfg, nil
}

// Disk returns a storage disk by name.
// Returns the default disk if name is empty.
func Disk(name string) *DiskWrapper {
	return &DiskWrapper{name: name}
}

// DiskWrapper provides a fluent API for storage operations.
type DiskWrapper struct {
	name string
}

func (d *DiskWrapper) storage() (Storage, error) {
	defaultMu.RLock()
	mgr := defaultMgr
	defaultMu.RUnlock()

	if mgr == nil {
		return nil, fmt.Errorf("storage: not initialized (call Setup first)")
	}
	return mgr.Disk(d.name)
}

// Storage returns the underlying Storage interface.
// Use this to access AdvancedStorage features like SignedURL, List, etc.
//
// Example:
//
//	s, err := storage.Disk("aliyun").Storage()
//	if adv, ok := s.(storage.AdvancedStorage); ok {
//	    url, _ := adv.SignedURL(ctx, "file.txt", time.Hour)
//	}
func (d *DiskWrapper) Storage() (Storage, error) {
	return d.storage()
}

// Put uploads data to the storage.
func (d *DiskWrapper) Put(key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error) {
	s, err := d.storage()
	if err != nil {
		return nil, err
	}
	return s.Upload(context.Background(), key, reader, opts...)
}

// PutWithContext uploads data with context.
func (d *DiskWrapper) PutWithContext(ctx context.Context, key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error) {
	s, err := d.storage()
	if err != nil {
		return nil, err
	}
	return s.Upload(ctx, key, reader, opts...)
}

// Get downloads data from the storage.
func (d *DiskWrapper) Get(key string) (io.ReadCloser, error) {
	s, err := d.storage()
	if err != nil {
		return nil, err
	}
	return s.Download(context.Background(), key)
}

// GetWithContext downloads data with context.
func (d *DiskWrapper) GetWithContext(ctx context.Context, key string) (io.ReadCloser, error) {
	s, err := d.storage()
	if err != nil {
		return nil, err
	}
	return s.Download(ctx, key)
}

// Delete removes a file from the storage.
func (d *DiskWrapper) Delete(key string) error {
	s, err := d.storage()
	if err != nil {
		return err
	}
	return s.Delete(context.Background(), key)
}

// Exists checks if a file exists.
func (d *DiskWrapper) Exists(key string) (bool, error) {
	s, err := d.storage()
	if err != nil {
		return false, err
	}
	return s.Exists(context.Background(), key)
}

// URL returns the public URL of a file.
func (d *DiskWrapper) URL(key string) (string, error) {
	s, err := d.storage()
	if err != nil {
		return "", err
	}
	return s.URL(context.Background(), key)
}

// PutFile uploads a file from local path.
func (d *DiskWrapper) PutFile(key, filePath string, opts ...UploadOption) (*UploadResult, error) {
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
		if ct := DetectContentType(filePath); ct != "" {
			opts = append(opts, WithContentType(ct))
		}
	}

	return d.Put(key, f, opts...)
}

// PutBytes uploads bytes directly.
func (d *DiskWrapper) PutBytes(key string, data []byte, opts ...UploadOption) (*UploadResult, error) {
	return d.Put(key, bytes.NewReader(data), opts...)
}

// PutString uploads a string directly.
func (d *DiskWrapper) PutString(key, content string, opts ...UploadOption) (*UploadResult, error) {
	return d.Put(key, strings.NewReader(content), opts...)
}

// GetBytes downloads and returns bytes.
func (d *DiskWrapper) GetBytes(key string) ([]byte, error) {
	reader, err := d.Get(key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// GetString downloads and returns string.
func (d *DiskWrapper) GetString(key string) (string, error) {
	data, err := d.GetBytes(key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MustPut uploads and panics on error.
func (d *DiskWrapper) MustPut(key string, reader io.Reader, opts ...UploadOption) *UploadResult {
	result, err := d.Put(key, reader, opts...)
	if err != nil {
		panic(err)
	}
	return result
}

// MustGet downloads and panics on error.
func (d *DiskWrapper) MustGet(key string) io.ReadCloser {
	reader, err := d.Get(key)
	if err != nil {
		panic(err)
	}
	return reader
}

// --- Package-level shortcuts using default disk ---

// Put uploads to the default disk.
func Put(key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error) {
	return Disk("").Put(key, reader, opts...)
}

// Get downloads from the default disk.
func Get(key string) (io.ReadCloser, error) {
	return Disk("").Get(key)
}

// Delete removes from the default disk.
func Delete(key string) error {
	return Disk("").Delete(key)
}

// Exists checks on the default disk.
func Exists(key string) (bool, error) {
	return Disk("").Exists(key)
}

// URL returns URL from the default disk.
func URL(key string) (string, error) {
	return Disk("").URL(key)
}

// --- Convenience functions ---

// PutFile uploads a file from local path.
func PutFile(key, filePath string, opts ...UploadOption) (*UploadResult, error) {
	return Disk("").PutFile(key, filePath, opts...)
}

// PutBytes uploads bytes directly.
func PutBytes(key string, data []byte, opts ...UploadOption) (*UploadResult, error) {
	return Disk("").PutBytes(key, data, opts...)
}

// PutString uploads a string directly.
func PutString(key, content string, opts ...UploadOption) (*UploadResult, error) {
	return Disk("").PutString(key, content, opts...)
}

// GetBytes downloads and returns bytes.
func GetBytes(key string) ([]byte, error) {
	return Disk("").GetBytes(key)
}

// GetString downloads and returns string.
func GetString(key string) (string, error) {
	return Disk("").GetString(key)
}
