// Package s3 provides AWS S3 compatible storage driver.
// Works with AWS S3, MinIO, and other S3-compatible services.
package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("s3", New)
	storage.Register("minio", New)
	storage.Register("aws", New)
}

// S3 implements storage.Storage for AWS S3 and compatible services.
type S3 struct {
	client   *s3.Client
	presign  *s3.PresignClient
	cfg      *Config
}

// Config for S3 storage.
type Config struct {
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // Custom endpoint for MinIO, etc.
	ForcePathStyle  bool   // Use path-style URLs (required for MinIO)
	Domain          string // Custom domain for URLs
}

// New creates a new S3 storage instance.
func New(cfg map[string]any) (storage.Storage, error) {
	c := &Config{}

	c.Region = getString(cfg, "region", "AWS_REGION", "S3_REGION")
	c.Bucket = getString(cfg, "bucket", "AWS_S3_BUCKET", "S3_BUCKET")
	c.AccessKeyID = getString(cfg, "access_key_id", "AWS_ACCESS_KEY_ID", "S3_ACCESS_KEY_ID")
	c.SecretAccessKey = getString(cfg, "secret_access_key", "AWS_SECRET_ACCESS_KEY", "S3_SECRET_ACCESS_KEY")
	c.Endpoint, _ = cfg["endpoint"].(string)
	c.ForcePathStyle, _ = cfg["force_path_style"].(bool)
	c.Domain, _ = cfg["domain"].(string)

	if c.Region == "" {
		c.Region = "us-east-1"
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}

	ctx := context.Background()
	var awsCfg aws.Config
	var err error

	if c.AccessKeyID != "" && c.SecretAccessKey != "" {
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(c.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				c.AccessKeyID, c.SecretAccessKey, "",
			)),
		)
	} else {
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(c.Region))
	}
	if err != nil {
		return nil, fmt.Errorf("s3: failed to load config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if c.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(c.Endpoint)
		})
	}
	if c.ForcePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	presign := s3.NewPresignClient(client)

	return &S3{
		client:  client,
		presign: presign,
		cfg:     c,
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

func (s *S3) Upload(ctx context.Context, key string, reader io.Reader, opts ...storage.UploadOption) (*storage.UploadResult, error) {
	options := &storage.UploadOptions{}
	for _, opt := range opts {
		opt(options)
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
		Body:   reader,
	}

	if options.ContentType != "" {
		input.ContentType = aws.String(options.ContentType)
	}
	if options.ContentDisposition != "" {
		input.ContentDisposition = aws.String(options.ContentDisposition)
	}
	if options.ACL != "" {
		input.ACL = s3types.ObjectCannedACL(options.ACL)
	}
	if len(options.Metadata) > 0 {
		input.Metadata = options.Metadata
	}

	resp, err := s.client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("s3: upload failed: %w", err)
	}

	result := &storage.UploadResult{Key: key}
	if resp.ETag != nil {
		result.ETag = *resp.ETag
	}
	if url, err := s.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

func (s *S3) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: download failed: %w", err)
	}
	return resp.Body, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3: delete failed: %w", err)
	}
	return nil
}

func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil // Assume not found
	}
	return true, nil
}

func (s *S3) URL(ctx context.Context, key string) (string, error) {
	if s.cfg.Domain != "" {
		return fmt.Sprintf("%s/%s", s.cfg.Domain, key), nil
	}
	if s.cfg.Endpoint != "" {
		if s.cfg.ForcePathStyle {
			return fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, key), nil
		}
		return fmt.Sprintf("%s/%s", s.cfg.Endpoint, key), nil
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.cfg.Bucket, s.cfg.Region, key), nil
}

func (s *S3) Close() error {
	return nil
}

// --- AdvancedStorage ---

func (s *S3) SignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("s3: failed to generate signed URL: %w", err)
	}
	return req.URL, nil
}

func (s *S3) List(ctx context.Context, prefix string, opts ...storage.ListOption) (*storage.ListResult, error) {
	options := &storage.ListOptions{MaxKeys: 1000}
	for _, opt := range opts {
		opt(options)
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.cfg.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(options.MaxKeys)),
	}
	if options.Marker != "" {
		input.StartAfter = aws.String(options.Marker)
	}
	if options.Delimiter != "" {
		input.Delimiter = aws.String(options.Delimiter)
	}

	resp, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("s3: list failed: %w", err)
	}

	var files []storage.FileInfo
	for _, obj := range resp.Contents {
		files = append(files, storage.FileInfo{
			Key:          *obj.Key,
			Size:         *obj.Size,
			LastModified: *obj.LastModified,
			ETag:         *obj.ETag,
		})
	}

	var nextMarker string
	if resp.NextContinuationToken != nil {
		nextMarker = *resp.NextContinuationToken
	}

	return &storage.ListResult{
		Files:       files,
		NextMarker:  nextMarker,
		IsTruncated: *resp.IsTruncated,
	}, nil
}

func (s *S3) Copy(ctx context.Context, src, dst string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.cfg.Bucket),
		Key:        aws.String(dst),
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.cfg.Bucket, src)),
	})
	if err != nil {
		return fmt.Errorf("s3: copy failed: %w", err)
	}
	return nil
}

func (s *S3) Move(ctx context.Context, src, dst string) error {
	if err := s.Copy(ctx, src, dst); err != nil {
		return err
	}
	return s.Delete(ctx, src)
}

func (s *S3) Size(ctx context.Context, key string) (int64, error) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("s3: failed to get size: %w", err)
	}
	return *resp.ContentLength, nil
}

func (s *S3) Metadata(ctx context.Context, key string) (*storage.FileInfo, error) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: failed to get metadata: %w", err)
	}

	info := &storage.FileInfo{
		Key:  key,
		Size: *resp.ContentLength,
	}
	if resp.ContentType != nil {
		info.ContentType = *resp.ContentType
	}
	if resp.ETag != nil {
		info.ETag = *resp.ETag
	}
	if resp.LastModified != nil {
		info.LastModified = *resp.LastModified
	}

	return info, nil
}

var _ storage.AdvancedStorage = (*S3)(nil)
