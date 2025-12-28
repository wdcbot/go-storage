// Package upyun provides Upyun storage driver.
package upyun

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/upyun/go-sdk/v3/upyun"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("upyun", New)
}

// Upyun implements storage.Storage for Upyun.
type Upyun struct {
	client *upyun.UpYun
	config *Config
}

// Config for Upyun storage.
type Config struct {
	Bucket   string
	Operator string
	Password string
	Domain   string // CDN domain for accessing files
}

// New creates a new Upyun storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.Bucket = getStringOrEnv(cfg, "bucket", "UPYUN_BUCKET")
	c.Operator = getStringOrEnv(cfg, "operator", "UPYUN_OPERATOR")
	c.Password = getStringOrEnv(cfg, "password", "UPYUN_PASSWORD")
	c.Domain = getStringOrEnv(cfg, "domain", "UPYUN_DOMAIN")

	if c.Bucket == "" {
		return nil, fmt.Errorf("upyun: bucket is required")
	}
	if c.Operator == "" {
		return nil, fmt.Errorf("upyun: operator is required")
	}
	if c.Password == "" {
		return nil, fmt.Errorf("upyun: password is required")
	}
	if c.Domain == "" {
		return nil, fmt.Errorf("upyun: domain is required")
	}

	client := upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   c.Bucket,
		Operator: c.Operator,
		Password: c.Password,
	})

	return &Upyun{
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

// Upload uploads a file to Upyun.
func (u *Upyun) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Ensure key starts with /
	if key[0] != '/' {
		key = "/" + key
	}

	putOpt := &upyun.PutObjectConfig{
		Path:   key,
		Reader: reader,
	}

	if options.ContentType != "" {
		putOpt.Headers = map[string]string{
			"Content-Type": options.ContentType,
		}
	}

	if err := u.client.Put(putOpt); err != nil {
		return nil, fmt.Errorf("upyun: upload failed: %w", err)
	}

	result := &storage.UploadResult{
		Key: key,
	}

	if url, err := u.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Upyun.
func (u *Upyun) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if key[0] != '/' {
		key = "/" + key
	}

	pr, pw := io.Pipe()

	go func() {
		_, err := u.client.Get(&upyun.GetObjectConfig{
			Path:   key,
			Writer: pw,
		})
		if err != nil {
			pw.CloseWithError(fmt.Errorf("upyun: download failed: %w", err))
		} else {
			pw.Close()
		}
	}()

	return pr, nil
}

// Delete deletes a file from Upyun.
func (u *Upyun) Delete(ctx context.Context, key string) error {
	if key[0] != '/' {
		key = "/" + key
	}

	if err := u.client.Delete(&upyun.DeleteObjectConfig{
		Path: key,
	}); err != nil {
		return fmt.Errorf("upyun: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Upyun.
func (u *Upyun) Exists(ctx context.Context, key string) (bool, error) {
	if key[0] != '/' {
		key = "/" + key
	}

	_, err := u.client.GetInfo(key)
	if err != nil {
		// Upyun returns error for non-existent files
		return false, nil
	}
	return true, nil
}

// URL returns the public URL of a file.
func (u *Upyun) URL(ctx context.Context, key string) (string, error) {
	if key[0] == '/' {
		key = key[1:]
	}
	return fmt.Sprintf("%s/%s", u.config.Domain, key), nil
}

// Close is a no-op for Upyun.
func (u *Upyun) Close() error {
	return nil
}
