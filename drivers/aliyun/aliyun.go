// Package aliyun provides Alibaba Cloud OSS storage driver.
package aliyun

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("aliyun", New)
	storage.Register("alioss", New)
	storage.Register("oss", New)
}

// Aliyun implements storage.Storage for Alibaba Cloud OSS.
type Aliyun struct {
	client *oss.Client
	bucket *oss.Bucket
	config *Config
}

// Config for Aliyun OSS.
type Config struct {
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	Bucket          string
	Domain          string // Custom domain (optional)
}

// New creates a new Aliyun OSS storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.Endpoint = getString(cfg, "endpoint", "ALIYUN_OSS_ENDPOINT", "OSS_ENDPOINT")
	c.AccessKeyID = getString(cfg, "access_key_id", "ALIYUN_ACCESS_KEY_ID", "OSS_ACCESS_KEY_ID")
	c.AccessKeySecret = getString(cfg, "access_key_secret", "ALIYUN_ACCESS_KEY_SECRET", "OSS_ACCESS_KEY_SECRET")
	c.Bucket = getString(cfg, "bucket", "ALIYUN_OSS_BUCKET", "OSS_BUCKET")
	c.Domain, _ = cfg["domain"].(string)

	if c.Endpoint == "" {
		return nil, fmt.Errorf("aliyun: endpoint is required")
	}
	if c.AccessKeyID == "" {
		return nil, fmt.Errorf("aliyun: access_key_id is required")
	}
	if c.AccessKeySecret == "" {
		return nil, fmt.Errorf("aliyun: access_key_secret is required")
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("aliyun: bucket is required")
	}

	client, err := oss.New(c.Endpoint, c.AccessKeyID, c.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("aliyun: failed to create client: %w", err)
	}

	bucket, err := client.Bucket(c.Bucket)
	if err != nil {
		return nil, fmt.Errorf("aliyun: failed to get bucket: %w", err)
	}

	return &Aliyun{
		client: client,
		bucket: bucket,
		config: c,
	}, nil
}

// getString gets string from config or env vars.
func getString(cfg map[string]any, key string, envKeys ...string) string {
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	for _, envKey := range envKeys {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
	}
	return ""
}

// Upload uploads a file to Aliyun OSS.
func (a *Aliyun) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	var ossOpts []oss.Option
	if options.ContentType != "" {
		ossOpts = append(ossOpts, oss.ContentType(options.ContentType))
	}
	if options.ContentDisposition != "" {
		ossOpts = append(ossOpts, oss.ContentDisposition(options.ContentDisposition))
	}
	for k, v := range options.Metadata {
		ossOpts = append(ossOpts, oss.Meta(k, v))
	}
	if options.ACL != "" {
		ossOpts = append(ossOpts, oss.ObjectACL(oss.ACLType(options.ACL)))
	}

	if err := a.bucket.PutObject(key, reader, ossOpts...); err != nil {
		return nil, fmt.Errorf("aliyun: upload failed: %w", err)
	}

	result := &storage.UploadResult{Key: key}
	if url, err := a.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Aliyun OSS.
func (a *Aliyun) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	body, err := a.bucket.GetObject(key)
	if err != nil {
		return nil, fmt.Errorf("aliyun: download failed: %w", err)
	}
	return body, nil
}

// Delete deletes a file from Aliyun OSS.
func (a *Aliyun) Delete(ctx context.Context, key string) error {
	if err := a.bucket.DeleteObject(key); err != nil {
		return fmt.Errorf("aliyun: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Aliyun OSS.
func (a *Aliyun) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := a.bucket.IsObjectExist(key)
	if err != nil {
		return false, fmt.Errorf("aliyun: exists check failed: %w", err)
	}
	return exists, nil
}

// URL returns the public URL of a file.
func (a *Aliyun) URL(ctx context.Context, key string) (string, error) {
	if a.config.Domain != "" {
		return fmt.Sprintf("%s/%s", a.config.Domain, key), nil
	}
	return fmt.Sprintf("https://%s.%s/%s", a.config.Bucket, a.config.Endpoint, key), nil
}

// Close is a no-op for Aliyun OSS.
func (a *Aliyun) Close() error {
	return nil
}

// --- AdvancedStorage ---

// SignedURL generates a pre-signed URL for temporary access.
func (a *Aliyun) SignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	url, err := a.bucket.SignURL(key, oss.HTTPGet, int64(expires.Seconds()))
	if err != nil {
		return "", fmt.Errorf("aliyun: failed to generate signed URL: %w", err)
	}
	return url, nil
}

// List lists files with the given prefix.
func (a *Aliyun) List(ctx context.Context, prefix string, opts ...storage.ListOption) (*storage.ListResult, error) {
	options := &storage.ListOptions{MaxKeys: 1000}
	for _, opt := range opts {
		opt(options)
	}

	listOpts := []oss.Option{
		oss.Prefix(prefix),
		oss.MaxKeys(options.MaxKeys),
	}
	if options.Marker != "" {
		listOpts = append(listOpts, oss.Marker(options.Marker))
	}
	if options.Delimiter != "" {
		listOpts = append(listOpts, oss.Delimiter(options.Delimiter))
	}

	lor, err := a.bucket.ListObjects(listOpts...)
	if err != nil {
		return nil, fmt.Errorf("aliyun: list failed: %w", err)
	}

	var files []storage.FileInfo
	for _, obj := range lor.Objects {
		files = append(files, storage.FileInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
		})
	}

	return &storage.ListResult{
		Files:       files,
		NextMarker:  lor.NextMarker,
		IsTruncated: lor.IsTruncated,
	}, nil
}

// Copy copies a file from src to dst.
func (a *Aliyun) Copy(ctx context.Context, src, dst string) error {
	_, err := a.bucket.CopyObject(src, dst)
	if err != nil {
		return fmt.Errorf("aliyun: copy failed: %w", err)
	}
	return nil
}

// Move moves a file from src to dst.
func (a *Aliyun) Move(ctx context.Context, src, dst string) error {
	if err := a.Copy(ctx, src, dst); err != nil {
		return err
	}
	return a.Delete(ctx, src)
}

// Size returns the size of a file.
func (a *Aliyun) Size(ctx context.Context, key string) (int64, error) {
	meta, err := a.bucket.GetObjectDetailedMeta(key)
	if err != nil {
		return 0, fmt.Errorf("aliyun: failed to get size: %w", err)
	}
	size := meta.Get("Content-Length")
	var s int64
	fmt.Sscanf(size, "%d", &s)
	return s, nil
}

// Metadata returns the metadata of a file.
func (a *Aliyun) Metadata(ctx context.Context, key string) (*storage.FileInfo, error) {
	meta, err := a.bucket.GetObjectDetailedMeta(key)
	if err != nil {
		return nil, fmt.Errorf("aliyun: failed to get metadata: %w", err)
	}

	var size int64
	fmt.Sscanf(meta.Get("Content-Length"), "%d", &size)

	return &storage.FileInfo{
		Key:         key,
		Size:        size,
		ContentType: meta.Get("Content-Type"),
		ETag:        meta.Get("ETag"),
	}, nil
}

// Ensure Aliyun implements AdvancedStorage
var _ storage.AdvancedStorage = (*Aliyun)(nil)
