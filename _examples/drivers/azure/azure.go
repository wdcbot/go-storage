// Package azure provides Azure Blob Storage driver.
package azure

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("azure", New)
	storage.Register("azblob", New) // Alias
}

// Azure implements storage.Storage for Azure Blob Storage.
type Azure struct {
	client    *azblob.Client
	container string
	config    *Config
}

// Config for Azure Blob Storage.
type Config struct {
	AccountName   string
	AccountKey    string
	Container     string
	Endpoint      string // Custom endpoint (optional)
	Domain        string // Custom domain for URLs (optional)
}

// New creates a new Azure Blob Storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.AccountName = getStringOrEnv(cfg, "account_name", "AZURE_STORAGE_ACCOUNT")
	c.AccountKey = getStringOrEnv(cfg, "account_key", "AZURE_STORAGE_KEY")
	c.Container = getStringOrEnv(cfg, "container", "AZURE_STORAGE_CONTAINER")
	c.Endpoint, _ = cfg["endpoint"].(string)
	c.Domain, _ = cfg["domain"].(string)

	if c.AccountName == "" {
		return nil, fmt.Errorf("azure: account_name is required")
	}
	if c.AccountKey == "" {
		return nil, fmt.Errorf("azure: account_key is required")
	}
	if c.Container == "" {
		return nil, fmt.Errorf("azure: container is required")
	}

	// Build service URL
	serviceURL := c.Endpoint
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://%s.blob.core.windows.net/", c.AccountName)
	}

	// Create credential
	cred, err := azblob.NewSharedKeyCredential(c.AccountName, c.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("azure: failed to create credential: %w", err)
	}

	// Create client
	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: failed to create client: %w", err)
	}

	return &Azure{
		client:    client,
		container: c.Container,
		config:    c,
	}, nil
}

func getStringOrEnv(cfg map[string]any, key, envKey string) string {
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	return os.Getenv(envKey)
}

// Upload uploads a file to Azure Blob Storage.
func (a *Azure) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	uploadOpts := &blockblob.UploadStreamOptions{}
	if options.ContentType != "" {
		uploadOpts.HTTPHeaders = &blob.HTTPHeaders{
			BlobContentType: &options.ContentType,
		}
	}
	if len(options.Metadata) > 0 {
		uploadOpts.Metadata = options.Metadata
	}

	resp, err := a.client.UploadStream(ctx, a.container, key, reader, uploadOpts)
	if err != nil {
		return nil, fmt.Errorf("azure: upload failed: %w", err)
	}

	result := &storage.UploadResult{
		Key: key,
	}
	if resp.ETag != nil {
		result.ETag = string(*resp.ETag)
	}

	if url, err := a.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Azure Blob Storage.
func (a *Azure) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := a.client.DownloadStream(ctx, a.container, key, nil)
	if err != nil {
		return nil, fmt.Errorf("azure: download failed: %w", err)
	}
	return resp.Body, nil
}

// Delete deletes a file from Azure Blob Storage.
func (a *Azure) Delete(ctx context.Context, key string) error {
	_, err := a.client.DeleteBlob(ctx, a.container, key, nil)
	if err != nil {
		return fmt.Errorf("azure: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Azure Blob Storage.
func (a *Azure) Exists(ctx context.Context, key string) (bool, error) {
	pager := a.client.NewListBlobsFlatPager(a.container, &azblob.ListBlobsFlatOptions{
		Prefix: &key,
	})

	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("azure: exists check failed: %w", err)
		}
		for _, item := range resp.Segment.BlobItems {
			if *item.Name == key {
				return true, nil
			}
		}
	}
	return false, nil
}

// URL returns the public URL of a file.
func (a *Azure) URL(ctx context.Context, key string) (string, error) {
	if a.config.Domain != "" {
		return fmt.Sprintf("%s/%s", a.config.Domain, key), nil
	}
	endpoint := a.config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.blob.core.windows.net", a.config.AccountName)
	}
	return fmt.Sprintf("%s/%s/%s", endpoint, a.container, key), nil
}

// Close is a no-op for Azure Blob Storage.
func (a *Azure) Close() error {
	return nil
}
