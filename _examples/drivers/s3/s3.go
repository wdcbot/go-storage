// Package s3 provides AWS S3 compatible storage driver.
// Works with AWS S3, MinIO, and other S3-compatible services.
package s3

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	storage "github.com/wdcbot/go-storage"
)

func init() {
	storage.Register("s3", New)
	storage.Register("minio", New) // MinIO is S3-compatible
}

// S3 implements storage.Storage for AWS S3 and compatible services.
type S3 struct {
	client *s3.Client
	cfg    *Config
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

	c.Region = getStringOrEnv(cfg, "region", "AWS_REGION")
	c.Bucket = getStringOrEnv(cfg, "bucket", "AWS_S3_BUCKET")
	c.AccessKeyID = getStringOrEnv(cfg, "access_key_id", "AWS_ACCESS_KEY_ID")
	c.SecretAccessKey = getStringOrEnv(cfg, "secret_access_key", "AWS_SECRET_ACCESS_KEY")
	c.Endpoint, _ = cfg["endpoint"].(string)
	c.ForcePathStyle, _ = cfg["force_path_style"].(bool)
	c.Domain, _ = cfg["domain"].(string)

	if c.Region == "" {
		c.Region = "us-east-1" // Default region
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}

	// Build AWS config
	var awsCfg aws.Config
	var err error

	if c.AccessKeyID != "" && c.SecretAccessKey != "" {
		// Use explicit credentials
		awsCfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(c.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				c.AccessKeyID, c.SecretAccessKey, "",
			)),
		)
	} else {
		// Use default credential chain (env, shared config, IAM role, etc.)
		awsCfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(c.Region),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("s3: failed to load config: %w", err)
	}

	// Create S3 client with options
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

	return &S3{
		client: client,
		cfg:    c,
	}, nil
}

func getStringOrEnv(cfg map[string]any, key, envKey string) string {
	if v, ok := cfg[key].(string); ok && v != "" {
		return v
	}
	return os.Getenv(envKey)
}

// Upload uploads a file to S3.
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
		input.ACL = s3Types.ObjectCannedACL(options.ACL)
	}
	if len(options.Metadata) > 0 {
		input.Metadata = options.Metadata
	}

	resp, err := s.client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("s3: upload failed: %w", err)
	}

	result := &storage.UploadResult{
		Key: key,
	}
	if resp.ETag != nil {
		result.ETag = *resp.ETag
	}

	if url, err := s.URL(ctx, key); err == nil {
		result.URL = url
	}

	return result, nil
}

// Download downloads a file from S3.
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

// Delete deletes a file from S3.
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

// Exists checks if a file exists in S3.
func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}
	return true, nil
}

// URL returns the public URL of a file.
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

// Close is a no-op for S3.
func (s *S3) Close() error {
	return nil
}
