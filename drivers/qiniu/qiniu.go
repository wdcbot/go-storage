// Package qiniu provides Qiniu Cloud storage driver.
package qiniu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
	gostorage "github.com/wdcbot/go-storage"
)

func init() {
	gostorage.Register("qiniu", New)
	gostorage.Register("qn", New)
}

// Qiniu implements storage.Storage for Qiniu Cloud.
type Qiniu struct {
	mac       *auth.Credentials
	cfg       *storage.Config
	bucket    string
	domain    string
	private   bool
	bucketMgr *storage.BucketManager
	uploader  *storage.FormUploader
}

// Config for Qiniu storage.
type Config struct {
	AccessKey string
	SecretKey string
	Bucket    string
	Domain    string
	Region    string // z0=华东, z1=华北, z2=华南, na0=北美, as0=东南亚
	UseHTTPS  bool
	Private   bool
}

// New creates a new Qiniu storage instance.
func New(cfg map[string]any) (gostorage.Storage, error) {
	c := &Config{}

	c.AccessKey = getString(cfg, "access_key", "QINIU_ACCESS_KEY")
	c.SecretKey = getString(cfg, "secret_key", "QINIU_SECRET_KEY")
	c.Bucket = getString(cfg, "bucket", "QINIU_BUCKET")
	c.Domain = getString(cfg, "domain", "QINIU_DOMAIN")
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
		return nil, fmt.Errorf("qiniu: domain is required")
	}

	mac := auth.New(c.AccessKey, c.SecretKey)

	storageCfg := &storage.Config{UseHTTPS: c.UseHTTPS}
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
	}

	return &Qiniu{
		mac:       mac,
		cfg:       storageCfg,
		bucket:    c.Bucket,
		domain:    c.Domain,
		private:   c.Private,
		bucketMgr: storage.NewBucketManager(mac, storageCfg),
		uploader:  storage.NewFormUploader(storageCfg),
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

func (q *Qiniu) Upload(ctx context.Context, key string, reader io.Reader, opts ...gostorage.UploadOption) (*gostorage.UploadResult, error) {
	options := &gostorage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", q.bucket, key),
	}
	upToken := putPolicy.UploadToken(q.mac)

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

func (q *Qiniu) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	url, err := q.URL(ctx, key)
	if err != nil {
		return nil, err
	}

	if q.private {
		deadline := time.Now().Add(time.Hour).Unix()
		url = storage.MakePrivateURL(q.mac, q.domain, key, deadline)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("qiniu: download failed: %w", err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("qiniu: download failed with status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (q *Qiniu) Delete(ctx context.Context, key string) error {
	err := q.bucketMgr.Delete(q.bucket, key)
	if err != nil {
		return fmt.Errorf("qiniu: delete failed: %w", err)
	}
	return nil
}

func (q *Qiniu) Exists(ctx context.Context, key string) (bool, error) {
	_, err := q.bucketMgr.Stat(q.bucket, key)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (q *Qiniu) URL(ctx context.Context, key string) (string, error) {
	if q.private {
		deadline := time.Now().Add(time.Hour).Unix()
		return storage.MakePrivateURL(q.mac, q.domain, key, deadline), nil
	}
	return fmt.Sprintf("%s/%s", q.domain, key), nil
}

func (q *Qiniu) Close() error {
	return nil
}

// --- AdvancedStorage ---

func (q *Qiniu) SignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	deadline := time.Now().Add(expires).Unix()
	return storage.MakePrivateURL(q.mac, q.domain, key, deadline), nil
}

func (q *Qiniu) List(ctx context.Context, prefix string, opts ...gostorage.ListOption) (*gostorage.ListResult, error) {
	options := &gostorage.ListOptions{MaxKeys: 1000}
	for _, opt := range opts {
		opt(options)
	}

	entries, _, nextMarker, hasNext, err := q.bucketMgr.ListFiles(q.bucket, prefix, options.Delimiter, options.Marker, options.MaxKeys)
	if err != nil {
		return nil, fmt.Errorf("qiniu: list failed: %w", err)
	}

	var files []gostorage.FileInfo
	for _, entry := range entries {
		files = append(files, gostorage.FileInfo{
			Key:          entry.Key,
			Size:         entry.Fsize,
			LastModified: time.Unix(entry.PutTime/1e7, 0),
			ContentType:  entry.MimeType,
		})
	}

	return &gostorage.ListResult{
		Files:       files,
		NextMarker:  nextMarker,
		IsTruncated: hasNext,
	}, nil
}

func (q *Qiniu) Copy(ctx context.Context, src, dst string) error {
	err := q.bucketMgr.Copy(q.bucket, src, q.bucket, dst, true)
	if err != nil {
		return fmt.Errorf("qiniu: copy failed: %w", err)
	}
	return nil
}

func (q *Qiniu) Move(ctx context.Context, src, dst string) error {
	err := q.bucketMgr.Move(q.bucket, src, q.bucket, dst, true)
	if err != nil {
		return fmt.Errorf("qiniu: move failed: %w", err)
	}
	return nil
}

func (q *Qiniu) Size(ctx context.Context, key string) (int64, error) {
	info, err := q.bucketMgr.Stat(q.bucket, key)
	if err != nil {
		return 0, fmt.Errorf("qiniu: failed to get size: %w", err)
	}
	return info.Fsize, nil
}

func (q *Qiniu) Metadata(ctx context.Context, key string) (*gostorage.FileInfo, error) {
	info, err := q.bucketMgr.Stat(q.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("qiniu: failed to get metadata: %w", err)
	}

	return &gostorage.FileInfo{
		Key:          key,
		Size:         info.Fsize,
		LastModified: time.Unix(info.PutTime/1e7, 0),
		ContentType:  info.MimeType,
		ETag:         info.Hash,
	}, nil
}

var _ gostorage.AdvancedStorage = (*Qiniu)(nil)
