package storage

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// mockStorage is a simple mock for testing.
type mockStorage struct {
	files map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{files: make(map[string][]byte)}
}

func (m *mockStorage) Upload(ctx context.Context, key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	m.files[key] = data
	return &UploadResult{Key: key, Size: int64(len(data))}, nil
}

func (m *mockStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.files[key]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	delete(m.files, key)
	return nil
}

func (m *mockStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.files[key]
	return ok, nil
}

func (m *mockStorage) URL(ctx context.Context, key string) (string, error) {
	return "https://example.com/" + key, nil
}

func (m *mockStorage) Close() error {
	return nil
}

func TestMockStorage_Upload(t *testing.T) {
	s := newMockStorage()
	ctx := context.Background()

	result, err := s.Upload(ctx, "test.txt", strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if result.Key != "test.txt" {
		t.Errorf("Expected key 'test.txt', got %q", result.Key)
	}

	if result.Size != 11 {
		t.Errorf("Expected size 11, got %d", result.Size)
	}
}

func TestMockStorage_Download(t *testing.T) {
	s := newMockStorage()
	ctx := context.Background()

	// Upload first
	s.Upload(ctx, "test.txt", strings.NewReader("hello world"))

	// Download
	reader, err := s.Download(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	if string(data) != "hello world" {
		t.Errorf("Expected 'hello world', got %q", string(data))
	}
}

func TestMockStorage_Download_NotFound(t *testing.T) {
	s := newMockStorage()
	ctx := context.Background()

	_, err := s.Download(ctx, "nonexistent.txt")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestMockStorage_Exists(t *testing.T) {
	s := newMockStorage()
	ctx := context.Background()

	// Should not exist
	exists, _ := s.Exists(ctx, "test.txt")
	if exists {
		t.Error("File should not exist")
	}

	// Upload
	s.Upload(ctx, "test.txt", strings.NewReader("hello"))

	// Should exist now
	exists, _ = s.Exists(ctx, "test.txt")
	if !exists {
		t.Error("File should exist")
	}
}

func TestMockStorage_Delete(t *testing.T) {
	s := newMockStorage()
	ctx := context.Background()

	s.Upload(ctx, "test.txt", strings.NewReader("hello"))
	s.Delete(ctx, "test.txt")

	exists, _ := s.Exists(ctx, "test.txt")
	if exists {
		t.Error("File should be deleted")
	}
}

func TestUploadOptions(t *testing.T) {
	opts := &UploadOptions{}

	WithContentType("image/png")(opts)
	if opts.ContentType != "image/png" {
		t.Errorf("Expected 'image/png', got %q", opts.ContentType)
	}

	WithACL("public-read")(opts)
	if opts.ACL != "public-read" {
		t.Errorf("Expected 'public-read', got %q", opts.ACL)
	}

	WithMetadata(map[string]string{"foo": "bar"})(opts)
	if opts.Metadata["foo"] != "bar" {
		t.Error("Metadata not set correctly")
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"image.png", "image/png"},
		{"doc.pdf", "application/pdf"},
		{"data.json", "application/json"},
		{"style.css", "text/css; charset=utf-8"},
		{"script.js", "application/javascript"},
		{"readme.md", "text/markdown"},
		{"config.yaml", "text/yaml"},
		{"unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := DetectContentType(tt.filename)
			if got != tt.expected {
				t.Errorf("DetectContentType(%q) = %q, want %q", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestGenerateKey(t *testing.T) {
	key := GenerateKey("images", "photo.jpg")

	if !strings.HasPrefix(key, "images/") {
		t.Errorf("Key should start with 'images/', got %q", key)
	}

	if !strings.HasSuffix(key, ".jpg") {
		t.Errorf("Key should end with '.jpg', got %q", key)
	}

	// Should contain date path
	now := time.Now()
	datePath := now.Format("2006/01/02")
	if !strings.Contains(key, datePath) {
		t.Errorf("Key should contain date path %q, got %q", datePath, key)
	}
}

func TestGenerateKeyFlat(t *testing.T) {
	key := GenerateKeyFlat("uploads", "doc.pdf")

	if !strings.HasPrefix(key, "uploads/") {
		t.Errorf("Key should start with 'uploads/', got %q", key)
	}

	if !strings.HasSuffix(key, ".pdf") {
		t.Errorf("Key should end with '.pdf', got %q", key)
	}

	// Should NOT contain date path
	if strings.Count(key, "/") > 1 {
		t.Errorf("Flat key should have only one slash, got %q", key)
	}
}

func TestIsNotExist(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{ErrNotFound, true},
		{NewError("test", "download", "key", ErrNotFound), true},
	}

	for _, tt := range tests {
		got := IsNotExist(tt.err)
		if got != tt.expected {
			t.Errorf("IsNotExist(%v) = %v, want %v", tt.err, got, tt.expected)
		}
	}
}

func TestRetry(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, 3, func() error {
		attempts++
		if attempts < 3 {
			return ErrNotFound
		}
		return nil
	})

	if err != nil {
		t.Errorf("Retry should succeed, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_MaxExceeded(t *testing.T) {
	ctx := context.Background()

	err := Retry(ctx, 2, func() error {
		return ErrNotFound
	})

	if err == nil {
		t.Error("Retry should fail after max attempts")
	}
}

func TestProgressReader(t *testing.T) {
	var lastUploaded, lastTotal int64

	content := "hello world"
	pr := NewProgressReader(
		strings.NewReader(content),
		int64(len(content)),
		func(uploaded, total int64) {
			lastUploaded = uploaded
			lastTotal = total
		},
	)

	data, _ := io.ReadAll(pr)

	if string(data) != content {
		t.Errorf("Expected %q, got %q", content, string(data))
	}

	if lastUploaded != int64(len(content)) {
		t.Errorf("Expected uploaded %d, got %d", len(content), lastUploaded)
	}

	if lastTotal != int64(len(content)) {
		t.Errorf("Expected total %d, got %d", len(content), lastTotal)
	}
}
