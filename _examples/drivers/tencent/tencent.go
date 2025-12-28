// Package tencent provides Tencent Cloud COS storage driver.
package tencent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/tencentyun/cos-go-sdk-v5"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("tencent", New)
	storage.Register("cos", New) // Alias
}

// Tencent implements storage.Storage for Tencent Cloud COS.
type Tencent struct {
	client *cos.Client
	config *Config
}

// Config for Tencent COS.
type Config struct {
	SecretID  string
	SecretKey string
	Region    string
	Bucket    string
	Domain    string // Custom domain (optional)
}

// New creates a new Tencent COS storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.SecretID = getStringOrEnv(cfg, "secret_id", "TENCENT_SECRET_ID")
	c.SecretKey = getStringOrEnv(cfg, "secret_key", "TENCENT_SECRET_KEY")
	c.Region = getStringOrEnv(cfg, "region", "TENCENT_COS_REGION")
	c.Bucket = getStringOrEnv(cfg, "bucket", "TENCENT_COS_BUCKET")
	c.Domain, _ = cfg["domain"].(string)

	if c.SecretID == "" {
		return nil, fmt.Errorf("tencent: secret_id is required")
	}
	if c.SecretKey == "" {
		return nil, fmt.Errorf("tencent: secret_key is required")
	}
	if c.Region == "" {
		return nil, fmt.Errorf("tencent: region is required")
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("tencent: bucket is required")
	}

	// Build bucket URL
	bucketURL, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", c.Bucket, c.Region))

	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  c.SecretID,
			SecretKey: c.SecretKey,
		},
	})

	return &Tencent{
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

// Upload uploads a file to Tencent COS.
func (t *Tencent) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	putOpt := &cos.ObjectPutOptions{}
	if options.ContentType != "" {
		putOpt.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{
			ContentType: options.ContentType,
		}
	}
	if options.ACL != "" {
		if putOpt.ObjectPutHeaderOptions == nil {
			putOpt.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{}
		}
		putOpt.ObjectPutHeaderOptions.XCosACL = options.ACL
	}

	resp, err := t.client.Object.Put(ctx, key, reader, putOpt)
	if err != nil {
		return nil, fmt.Errorf("tencent: upload failed: %w", err)
	}
	defer resp.Body.Close()

	result := &storage.UploadResult{
		Key:  key,
		ETag: resp.Header.Get("ETag"),
	}

	if url, err := t.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Tencent COS.
func (t *Tencent) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := t.client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("tencent: download failed: %w", err)
	}
	return resp.Body, nil
}

// Delete deletes a file from Tencent COS.
func (t *Tencent) Delete(ctx context.Context, key string) error {
	_, err := t.client.Object.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("tencent: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Tencent COS.
func (t *Tencent) Exists(ctx context.Context, key string) (bool, error) {
	ok, err := t.client.Object.IsExist(ctx, key)
	if err != nil {
		return false, fmt.Errorf("tencent: exists check failed: %w", err)
	}
	return ok, nil
}

// URL returns the public URL of a file.
func (t *Tencent) URL(ctx context.Context, key string) (string, error) {
	if t.config.Domain != "" {
		return fmt.Sprintf("%s/%s", t.config.Domain, key), nil
	}
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s", t.config.Bucket, t.config.Region, key), nil
}

// Close is a no-op for Tencent COS.
func (t *Tencent) Close() error {
	return nil
}
