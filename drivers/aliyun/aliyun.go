// Package aliyun provides Alibaba Cloud OSS storage driver.
package aliyun

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("aliyun", New)
	storage.Register("alioss", New) // Alias
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
	PathStyle       bool   // Use path-style URLs
}

// New creates a new Aliyun OSS storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	// Support both config file and environment variables
	c.Endpoint = getStringOrEnv(cfg, "endpoint", "ALIYUN_OSS_ENDPOINT")
	c.AccessKeyID = getStringOrEnv(cfg, "access_key_id", "ALIYUN_ACCESS_KEY_ID")
	c.AccessKeySecret = getStringOrEnv(cfg, "access_key_secret", "ALIYUN_ACCESS_KEY_SECRET")
	c.Bucket = getStringOrEnv(cfg, "bucket", "ALIYUN_OSS_BUCKET")
	c.Domain, _ = cfg["domain"].(string)
	c.PathStyle, _ = cfg["path_style"].(bool)

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

func getStringOrEnv(cfg map[string]any, key, envKey string) string {
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	return os.Getenv(envKey)
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

	result := &storage.UploadResult{
		Key: key,
	}

	// Generate URL
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
	// Default OSS URL format
	return fmt.Sprintf("https://%s.%s/%s", a.config.Bucket, a.config.Endpoint, key), nil
}

// Close is a no-op for Aliyun OSS.
func (a *Aliyun) Close() error {
	return nil
}
