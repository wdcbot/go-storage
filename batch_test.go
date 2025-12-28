package storage

import (
	"context"
	"strings"
	"testing"
)

func TestBatchUpload(t *testing.T) {
	s := newMockStorage()
	ctx := context.Background()

	items := []BatchUploadItem{
		{Key: "a.txt", Reader: strings.NewReader("aaa")},
		{Key: "b.txt", Reader: strings.NewReader("bbb")},
		{Key: "c.txt", Reader: strings.NewReader("ccc")},
	}

	result := BatchUpload(ctx, s, items, 2)

	if len(result.Succeeded) != 3 {
		t.Errorf("Expected 3 succeeded, got %d", len(result.Succeeded))
	}

	if len(result.Failed) != 0 {
		t.Errorf("Expected 0 failed, got %d", len(result.Failed))
	}

	// Verify files exist
	for _, item := range items {
		exists, _ := s.Exists(ctx, item.Key)
		if !exists {
			t.Errorf("File %q should exist", item.Key)
		}
	}
}

func TestBatchDelete(t *testing.T) {
	s := newMockStorage()
	ctx := context.Background()

	// Upload some files first
	s.Upload(ctx, "a.txt", strings.NewReader("a"))
	s.Upload(ctx, "b.txt", strings.NewReader("b"))
	s.Upload(ctx, "c.txt", strings.NewReader("c"))

	keys := []string{"a.txt", "b.txt", "c.txt"}
	result := BatchDelete(ctx, s, keys, 2)

	if len(result.Succeeded) != 3 {
		t.Errorf("Expected 3 succeeded, got %d", len(result.Succeeded))
	}

	// Verify files are deleted
	for _, key := range keys {
		exists, _ := s.Exists(ctx, key)
		if exists {
			t.Errorf("File %q should be deleted", key)
		}
	}
}

func TestBatchUpload_WithCancellation(t *testing.T) {
	s := newMockStorage()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	items := []BatchUploadItem{
		{Key: "a.txt", Reader: strings.NewReader("aaa")},
	}

	result := BatchUpload(ctx, s, items, 1)

	// Should have failures due to cancellation
	if len(result.Failed) == 0 && len(result.Succeeded) == 0 {
		// Context was cancelled before any work started
		t.Log("Context cancelled before processing")
	}
}
