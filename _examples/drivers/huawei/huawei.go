// Package huawei provides Huawei Cloud OBS storage driver.
package huawei

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("huawei", New)
	storage.Register("obs", New) // Alias
}

// Huawei implements storage.Storage for Huawei Cloud OBS.
type Huawei struct {
	client *obs.ObsClient
	config *Config
}

// Config for Huawei OBS.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Domain    string // Custom domain (optional)
}

// New creates a new Huawei OBS storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.Endpoint = getStringOrEnv(cfg, "endpoint", "HUAWEI_OBS_ENDPOINT")
	c.AccessKey = getStringOrEnv(cfg, "access_key", "HUAWEI_ACCESS_KEY")
	c.SecretKey = getStringOrEnv(cfg, "secret_key", "HUAWEI_SECRET_KEY")
	c.Bucket = getStringOrEnv(cfg, "bucket", "HUAWEI_OBS_BUCKET")
	c.Domain, _ = cfg["domain"].(string)

	if c.Endpoint == "" {
		return nil, fmt.Errorf("huawei: endpoint is required")
	}
	if c.AccessKey == "" {
		return nil, fmt.Errorf("huawei: access_key is required")
	}
	if c.SecretKey == "" {
		return nil, fmt.Errorf("huawei: secret_key is required")
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("huawei: bucket is required")
	}

	client, err := obs.New(c.AccessKey, c.SecretKey, c.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("huawei: failed to create client: %w", err)
	}

	return &Huawei{
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

// Upload uploads a file to Huawei OBS.
func (h *Huawei) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	input := &obs.PutObjectInput{}
	input.Bucket = h.config.Bucket
	input.Key = key
	input.Body = reader

	if options.ContentType != "" {
		input.ContentType = options.ContentType
	}
	if options.ACL != "" {
		input.ACL = obs.AclType(options.ACL)
	}
	if len(options.Metadata) > 0 {
		input.Metadata = options.Metadata
	}

	output, err := h.client.PutObject(input)
	if err != nil {
		return nil, fmt.Errorf("huawei: upload failed: %w", err)
	}

	result := &storage.UploadResult{
		Key:  key,
		ETag: output.ETag,
	}

	if url, err := h.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Huawei OBS.
func (h *Huawei) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &obs.GetObjectInput{}
	input.Bucket = h.config.Bucket
	input.Key = key

	output, err := h.client.GetObject(input)
	if err != nil {
		return nil, fmt.Errorf("huawei: download failed: %w", err)
	}

	return output.Body, nil
}

// Delete deletes a file from Huawei OBS.
func (h *Huawei) Delete(ctx context.Context, key string) error {
	input := &obs.DeleteObjectInput{}
	input.Bucket = h.config.Bucket
	input.Key = key

	_, err := h.client.DeleteObject(input)
	if err != nil {
		return fmt.Errorf("huawei: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Huawei OBS.
func (h *Huawei) Exists(ctx context.Context, key string) (bool, error) {
	input := &obs.GetObjectMetadataInput{}
	input.Bucket = h.config.Bucket
	input.Key = key

	_, err := h.client.GetObjectMetadata(input)
	if err != nil {
		// Check if it's a "not found" error
		if obsErr, ok := err.(obs.ObsError); ok && obsErr.StatusCode == 404 {
			return false, nil
		}
		return false, fmt.Errorf("huawei: exists check failed: %w", err)
	}
	return true, nil
}

// URL returns the public URL of a file.
func (h *Huawei) URL(ctx context.Context, key string) (string, error) {
	if h.config.Domain != "" {
		return fmt.Sprintf("%s/%s", h.config.Domain, key), nil
	}
	return fmt.Sprintf("https://%s.%s/%s", h.config.Bucket, h.config.Endpoint, key), nil
}

// Close closes the Huawei OBS client.
func (h *Huawei) Close() error {
	h.client.Close()
	return nil
}
