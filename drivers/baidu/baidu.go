// Package baidu provides Baidu Cloud BOS storage driver.
package baidu

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/baidubce/bce-sdk-go/bce"
	"github.com/baidubce/bce-sdk-go/services/bos"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("baidu", New)
	storage.Register("bos", New) // Alias
}

// Baidu implements storage.Storage for Baidu Cloud BOS.
type Baidu struct {
	client *bos.Client
	config *Config
}

// Config for Baidu BOS.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Domain    string // Custom domain (optional)
}

// New creates a new Baidu BOS storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.Endpoint = getStringOrEnv(cfg, "endpoint", "BAIDU_BOS_ENDPOINT")
	c.AccessKey = getStringOrEnv(cfg, "access_key", "BAIDU_ACCESS_KEY")
	c.SecretKey = getStringOrEnv(cfg, "secret_key", "BAIDU_SECRET_KEY")
	c.Bucket = getStringOrEnv(cfg, "bucket", "BAIDU_BOS_BUCKET")
	c.Domain, _ = cfg["domain"].(string)

	if c.Endpoint == "" {
		return nil, fmt.Errorf("baidu: endpoint is required")
	}
	if c.AccessKey == "" {
		return nil, fmt.Errorf("baidu: access_key is required")
	}
	if c.SecretKey == "" {
		return nil, fmt.Errorf("baidu: secret_key is required")
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("baidu: bucket is required")
	}

	clientConfig := bos.BosClientConfiguration{
		Ak:               c.AccessKey,
		Sk:               c.SecretKey,
		Endpoint:         c.Endpoint,
		RedirectDisabled: false,
	}

	client, err := bos.NewClientWithConfig(&clientConfig)
	if err != nil {
		return nil, fmt.Errorf("baidu: failed to create client: %w", err)
	}

	return &Baidu{
		client: client,
		config: c,
	}, nil
}

func getStringOrEnv(cfg map[string]any, key, envKey string) string {
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	return os.Getenv(envKey)
}

// Upload uploads a file to Baidu BOS.
func (b *Baidu) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Read all data since BOS SDK needs content length
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("baidu: failed to read data: %w", err)
	}

	args := new(bos.PutObjectArgs)
	if options.ContentType != "" {
		args.ContentType = options.ContentType
	}
	if options.ACL != "" {
		args.CannedAcl = options.ACL
	}
	if len(options.Metadata) > 0 {
		args.UserMeta = options.Metadata
	}

	etag, err := b.client.PutObjectFromBytes(b.config.Bucket, key, data, args)
	if err != nil {
		return nil, fmt.Errorf("baidu: upload failed: %w", err)
	}

	result := &storage.UploadResult{
		Key:  key,
		Size: int64(len(data)),
		ETag: etag,
	}

	if url, err := b.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Baidu BOS.
func (b *Baidu) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := b.client.BasicGetObject(b.config.Bucket, key)
	if err != nil {
		return nil, fmt.Errorf("baidu: download failed: %w", err)
	}
	return result.Body, nil
}

// Delete deletes a file from Baidu BOS.
func (b *Baidu) Delete(ctx context.Context, key string) error {
	if err := b.client.DeleteObject(b.config.Bucket, key); err != nil {
		return fmt.Errorf("baidu: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Baidu BOS.
func (b *Baidu) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.client.GetObjectMeta(b.config.Bucket, key)
	if err != nil {
		if realErr, ok := err.(*bce.BceServiceError); ok && realErr.StatusCode == 404 {
			return false, nil
		}
		return false, fmt.Errorf("baidu: exists check failed: %w", err)
	}
	return true, nil
}

// URL returns the public URL of a file.
func (b *Baidu) URL(ctx context.Context, key string) (string, error) {
	if b.config.Domain != "" {
		return fmt.Sprintf("%s/%s", b.config.Domain, key), nil
	}
	return fmt.Sprintf("https://%s.%s/%s", b.config.Bucket, b.config.Endpoint, key), nil
}

// Close is a no-op for Baidu BOS.
func (b *Baidu) Close() error {
	return nil
}
