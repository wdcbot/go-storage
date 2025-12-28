// Package tencent provides Tencent Cloud COS storage driver.
package tencent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("tencent", New)
	storage.Register("cos", New)
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
	Domain    string
}

// New creates a new Tencent COS storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.SecretID = getString(cfg, "secret_id", "TENCENT_SECRET_ID", "COS_SECRET_ID")
	c.SecretKey = getString(cfg, "secret_key", "TENCENT_SECRET_KEY", "COS_SECRET_KEY")
	c.Region = getString(cfg, "region", "TENCENT_COS_REGION", "COS_REGION")
	c.Bucket = getString(cfg, "bucket", "TENCENT_COS_BUCKET", "COS_BUCKET")
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

func (t *Tencent) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	putOpt := &cos.ObjectPutOptions{}
	if options.ContentType != "" || options.ACL != "" {
		putOpt.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{}
		if options.ContentType != "" {
			putOpt.ObjectPutHeaderOptions.ContentType = options.ContentType
		}
		if options.ACL != "" {
			putOpt.ObjectPutHeaderOptions.XCosACL = options.ACL
		}
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

func (t *Tencent) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := t.client.Object.Get(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("tencent: download failed: %w", err)
	}
	return resp.Body, nil
}

func (t *Tencent) Delete(ctx context.Context, key string) error {
	_, err := t.client.Object.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("tencent: delete failed: %w", err)
	}
	return nil
}

func (t *Tencent) Exists(ctx context.Context, key string) (bool, error) {
	ok, err := t.client.Object.IsExist(ctx, key)
	if err != nil {
		return false, fmt.Errorf("tencent: exists check failed: %w", err)
	}
	return ok, nil
}

func (t *Tencent) URL(ctx context.Context, key string) (string, error) {
	if t.config.Domain != "" {
		return fmt.Sprintf("%s/%s", t.config.Domain, key), nil
	}
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s", t.config.Bucket, t.config.Region, key), nil
}

func (t *Tencent) Close() error {
	return nil
}

// --- AdvancedStorage ---

func (t *Tencent) SignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	presignedURL, err := t.client.Object.GetPresignedURL(ctx, http.MethodGet, key, t.config.SecretID, t.config.SecretKey, expires, nil)
	if err != nil {
		return "", fmt.Errorf("tencent: failed to generate signed URL: %w", err)
	}
	return presignedURL.String(), nil
}

func (t *Tencent) List(ctx context.Context, prefix string, opts ...storage.ListOption) (*storage.ListResult, error) {
	options := &storage.ListOptions{MaxKeys: 1000}
	for _, opt := range opts {
		opt(options)
	}

	listOpt := &cos.BucketGetOptions{
		Prefix:    prefix,
		MaxKeys:   options.MaxKeys,
		Marker:    options.Marker,
		Delimiter: options.Delimiter,
	}

	result, _, err := t.client.Bucket.Get(ctx, listOpt)
	if err != nil {
		return nil, fmt.Errorf("tencent: list failed: %w", err)
	}

	var files []storage.FileInfo
	for _, obj := range result.Contents {
		files = append(files, storage.FileInfo{
			Key:  obj.Key,
			Size: int64(obj.Size),
			ETag: obj.ETag,
		})
	}

	return &storage.ListResult{
		Files:       files,
		NextMarker:  result.NextMarker,
		IsTruncated: result.IsTruncated,
	}, nil
}

func (t *Tencent) Copy(ctx context.Context, src, dst string) error {
	sourceURL := fmt.Sprintf("%s.cos.%s.myqcloud.com/%s", t.config.Bucket, t.config.Region, src)
	_, _, err := t.client.Object.Copy(ctx, dst, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("tencent: copy failed: %w", err)
	}
	return nil
}

func (t *Tencent) Move(ctx context.Context, src, dst string) error {
	if err := t.Copy(ctx, src, dst); err != nil {
		return err
	}
	return t.Delete(ctx, src)
}

func (t *Tencent) Size(ctx context.Context, key string) (int64, error) {
	resp, err := t.client.Object.Head(ctx, key, nil)
	if err != nil {
		return 0, fmt.Errorf("tencent: failed to get size: %w", err)
	}
	return resp.ContentLength, nil
}

func (t *Tencent) Metadata(ctx context.Context, key string) (*storage.FileInfo, error) {
	resp, err := t.client.Object.Head(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("tencent: failed to get metadata: %w", err)
	}

	return &storage.FileInfo{
		Key:         key,
		Size:        resp.ContentLength,
		ContentType: resp.Header.Get("Content-Type"),
		ETag:        resp.Header.Get("ETag"),
	}, nil
}

var _ storage.AdvancedStorage = (*Tencent)(nil)
