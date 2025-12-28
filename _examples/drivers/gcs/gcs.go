// Package gcs provides Google Cloud Storage driver.
package gcs

import (
	"context"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
	gostorage "github.com/wdcbot/go-storage"
)

func init() {
	gostorage.Register("gcs", New)
	gostorage.Register("google", New) // Alias
}

// GCS implements storage.Storage for Google Cloud Storage.
type GCS struct {
	client *storage.Client
	bucket *storage.BucketHandle
	config *Config
}

// Config for Google Cloud Storage.
type Config struct {
	Bucket          string
	CredentialsFile string // Path to service account JSON file
	CredentialsJSON string // Service account JSON content
	ProjectID       string
	Domain          string // Custom domain (optional)
}

// New creates a new Google Cloud Storage instance.
func New(cfg map[string]any) (gostorage.Storage, error) {
	c := &Config{}

	c.Bucket = getStringOrEnv(cfg, "bucket", "GCS_BUCKET")
	c.CredentialsFile = getStringOrEnv(cfg, "credentials_file", "GOOGLE_APPLICATION_CREDENTIALS")
	c.CredentialsJSON, _ = cfg["credentials_json"].(string)
	c.ProjectID, _ = cfg["project_id"].(string)
	c.Domain, _ = cfg["domain"].(string)

	if c.Bucket == "" {
		return nil, fmt.Errorf("gcs: bucket is required")
	}

	ctx := context.Background()
	var opts []option.ClientOption

	if c.CredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(c.CredentialsJSON)))
	} else if c.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(c.CredentialsFile))
	}
	// If neither is set, will use default credentials (ADC)

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcs: failed to create client: %w", err)
	}

	return &GCS{
		client: client,
		bucket: client.Bucket(c.Bucket),
		config: c,
	}, nil
}

func getStringOrEnv(cfg map[string]any, key, envKey string) string {
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	return os.Getenv(envKey)
}

// Upload uploads a file to Google Cloud Storage.
func (g *GCS) Upload(ctx context.Context, key string, reader io.Reader, opts ...gostorage.UploadOption) (*gostorage.UploadResult, error) {
	options := &gostorage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	obj := g.bucket.Object(key)
	writer := obj.NewWriter(ctx)

	if options.ContentType != "" {
		writer.ContentType = options.ContentType
	}
	if options.ContentDisposition != "" {
		writer.ContentDisposition = options.ContentDisposition
	}
	if len(options.Metadata) > 0 {
		writer.Metadata = options.Metadata
	}
	if options.ACL == "public-read" {
		writer.PredefinedACL = "publicRead"
	}

	size, err := io.Copy(writer, reader)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("gcs: upload failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gcs: upload failed: %w", err)
	}

	result := &gostorage.UploadResult{
		Key:  key,
		Size: size,
	}

	if url, err := g.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Google Cloud Storage.
func (g *GCS) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj := g.bucket.Object(key)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs: download failed: %w", err)
	}
	return reader, nil
}

// Delete deletes a file from Google Cloud Storage.
func (g *GCS) Delete(ctx context.Context, key string) error {
	obj := g.bucket.Object(key)
	if err := obj.Delete(ctx); err != nil {
		if err == storage.ErrObjectNotExist {
			return nil // Already deleted
		}
		return fmt.Errorf("gcs: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Google Cloud Storage.
func (g *GCS) Exists(ctx context.Context, key string) (bool, error) {
	obj := g.bucket.Object(key)
	_, err := obj.Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, fmt.Errorf("gcs: exists check failed: %w", err)
	}
	return true, nil
}

// URL returns the public URL of a file.
func (g *GCS) URL(ctx context.Context, key string) (string, error) {
	if g.config.Domain != "" {
		return fmt.Sprintf("%s/%s", g.config.Domain, key), nil
	}
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", g.config.Bucket, key), nil
}

// Close closes the GCS client.
func (g *GCS) Close() error {
	return g.client.Close()
}
