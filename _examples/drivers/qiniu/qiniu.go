// Package qiniu provides Qiniu Cloud storage driver.
package qiniu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
	gostorage "github.com/wdcbot/go-storage"
)

func init() {
	gostorage.Register("qiniu", New)
}

// Qiniu implements storage.Storage for Qiniu Cloud.
type Qiniu struct {
	mac        *auth.Credentials
	cfg        *storage.Config
	bucket     string
	domain     string
	bucketMgr  *storage.BucketManager
	uploader   *storage.FormUploader
	resumeUp   *storage.ResumeUploaderV2
}

// Config for Qiniu storage.
type Config struct {
	AccessKey string
	SecretKey string
	Bucket    string
	Domain    string // CDN domain for accessing files
	Region    string // e.g., "z0" (华东), "z1" (华北), "z2" (华南), "na0" (北美), "as0" (东南亚)
	UseHTTPS  bool
	Private   bool // Whether the bucket is private
}

// New creates a new Qiniu storage instance.
func New(cfg map[string]any) (gostorage.Storage, error) {
	c := &Config{}

	c.AccessKey = getStringOrEnv(cfg, "access_key", "QINIU_ACCESS_KEY")
	c.SecretKey = getStringOrEnv(cfg, "secret_key", "QINIU_SECRET_KEY")
	c.Bucket = getStringOrEnv(cfg, "bucket", "QINIU_BUCKET")
	c.Domain = getStringOrEnv(cfg, "domain", "QINIU_DOMAIN")
	c.Region, _ = cfg["region"].(string)
	c.UseHTTPS, _ = cfg["use_https"].(bool)
	c.Private, _ = cfg["private"].(bool)

	if c.AccessKey == "" {
		return nil, fmt.Errorf("qiniu: access_key is required")
	}
	if c.SecretKey == "" {
		return nil, fmt.Errorf("qiniu: secret_key is required")
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("qiniu: bucket is required")
	}
	if c.Domain == "" {
		return nil, fmt.Errorf("qiniu: domain is required for accessing files")
	}

	mac := auth.New(c.AccessKey, c.SecretKey)

	// Configure region
	storageCfg := &storage.Config{
		UseHTTPS: c.UseHTTPS,
	}
	switch c.Region {
	case "z0", "huadong":
		storageCfg.Region = &storage.ZoneHuadong
	case "z1", "huabei":
		storageCfg.Region = &storage.ZoneHuabei
	case "z2", "huanan":
		storageCfg.Region = &storage.ZoneHuanan
	case "na0", "beimei":
		storageCfg.Region = &storage.ZoneBeimei
	case "as0", "xinjiapo":
		storageCfg.Region = &storage.ZoneXinjiapo
	default:
		// Auto-detect region
		storageCfg.Region = nil
	}

	bucketMgr := storage.NewBucketManager(mac, storageCfg)
	uploader := storage.NewFormUploader(storageCfg)
	resumeUp := storage.NewResumeUploaderV2(storageCfg)

	return &Qiniu{
		mac:       mac,
		cfg:       storageCfg,
		bucket:    c.Bucket,
		domain:    c.Domain,
		bucketMgr: bucketMgr,
		uploader:  uploader,
		resumeUp:  resumeUp,
	}, nil
}

func getStringOrEnv(cfg map[string]any, key, envKey string) string {
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	return os.Getenv(envKey)
}

// Upload uploads a file to Qiniu.
func (q *Qiniu) Upload(ctx context.Context, key string, reader io.Reader, opts ...gostorage.UploadOption) (*gostorage.UploadResult, error) {
	options := &gostorage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Generate upload token
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", q.bucket, key),
	}
	upToken := putPolicy.UploadToken(q.mac)

	// Read all data (Qiniu SDK needs size)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("qiniu: failed to read data: %w", err)
	}

	ret := storage.PutRet{}
	putExtra := storage.PutExtra{}
	if options.ContentType != "" {
		putExtra.MimeType = options.ContentType
	}

	err = q.uploader.Put(ctx, &ret, upToken, key, bytes.NewReader(data), int64(len(data)), &putExtra)
	if err != nil {
		return nil, fmt.Errorf("qiniu: upload failed: %w", err)
	}

	result := &gostorage.UploadResult{
		Key:  ret.Key,
		Size: int64(len(data)),
	}

	if url, err := q.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from Qiniu.
func (q *Qiniu) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	url, err := q.URL(ctx, key)
	if err != nil {
		return nil, err
	}

	// Use HTTP client to download
	resp, err := storage.DefaultClient.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("qiniu: download failed: %w", err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("qiniu: download failed with status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// Delete deletes a file from Qiniu.
func (q *Qiniu) Delete(ctx context.Context, key string) error {
	err := q.bucketMgr.Delete(q.bucket, key)
	if err != nil {
		return fmt.Errorf("qiniu: delete failed: %w", err)
	}
	return nil
}

// Exists checks if a file exists in Qiniu.
func (q *Qiniu) Exists(ctx context.Context, key string) (bool, error) {
	_, err := q.bucketMgr.Stat(q.bucket, key)
	if err != nil {
		// Check if it's a "not found" error
		if err.Error() == "no such file or directory" {
			return false, nil
		}
		return false, fmt.Errorf("qiniu: exists check failed: %w", err)
	}
	return true, nil
}

// URL returns the public URL of a file.
func (q *Qiniu) URL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/%s", q.domain, key), nil
}

// Close is a no-op for Qiniu.
func (q *Qiniu) Close() error {
	return nil
}
