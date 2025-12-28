package storage

import (
	"context"
	"io"
	"sync"
)

// BatchUploadItem represents a single item in a batch upload.
type BatchUploadItem struct {
	Key    string
	Reader io.Reader
	Opts   []UploadOption
}

// BatchUploadResult contains results of a batch upload.
type BatchUploadResult struct {
	Succeeded []*UploadResult
	Failed    []BatchError
}

// BatchError represents an error for a single item in a batch operation.
type BatchError struct {
	Key string
	Err error
}

// BatchUpload uploads multiple files concurrently.
// concurrency controls how many uploads run in parallel (0 = no limit).
func BatchUpload(ctx context.Context, s Storage, items []BatchUploadItem, concurrency int) *BatchUploadResult {
	result := &BatchUploadResult{}
	var mu sync.Mutex

	if concurrency <= 0 {
		concurrency = len(items)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, item := range items {
		select {
		case <-ctx.Done():
			mu.Lock()
			result.Failed = append(result.Failed, BatchError{Key: item.Key, Err: ctx.Err()})
			mu.Unlock()
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(item BatchUploadItem) {
			defer wg.Done()
			defer func() { <-sem }()

			uploadResult, err := s.Upload(ctx, item.Key, item.Reader, item.Opts...)
			mu.Lock()
			if err != nil {
				result.Failed = append(result.Failed, BatchError{Key: item.Key, Err: err})
			} else {
				result.Succeeded = append(result.Succeeded, uploadResult)
			}
			mu.Unlock()
		}(item)
	}

	wg.Wait()
	return result
}

// BatchDeleteResult contains results of a batch delete.
type BatchDeleteResult struct {
	Succeeded []string
	Failed    []BatchError
}

// BatchDelete deletes multiple files concurrently.
func BatchDelete(ctx context.Context, s Storage, keys []string, concurrency int) *BatchDeleteResult {
	result := &BatchDeleteResult{}
	var mu sync.Mutex

	if concurrency <= 0 {
		concurrency = len(keys)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, key := range keys {
		select {
		case <-ctx.Done():
			mu.Lock()
			result.Failed = append(result.Failed, BatchError{Key: key, Err: ctx.Err()})
			mu.Unlock()
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			defer func() { <-sem }()

			err := s.Delete(ctx, key)
			mu.Lock()
			if err != nil {
				result.Failed = append(result.Failed, BatchError{Key: key, Err: err})
			} else {
				result.Succeeded = append(result.Succeeded, key)
			}
			mu.Unlock()
		}(key)
	}

	wg.Wait()
	return result
}

// DeleteAll deletes all files with the given prefix.
// Only works with AdvancedStorage that supports List.
func DeleteAll(ctx context.Context, s Storage, prefix string, concurrency int) (*BatchDeleteResult, error) {
	adv, ok := s.(AdvancedStorage)
	if !ok {
		return nil, ErrNotImplemented
	}

	var allKeys []string
	marker := ""

	for {
		opts := []ListOption{WithMaxKeys(1000)}
		if marker != "" {
			opts = append(opts, WithMarker(marker))
		}

		listResult, err := adv.List(ctx, prefix, opts...)
		if err != nil {
			return nil, err
		}

		for _, f := range listResult.Files {
			allKeys = append(allKeys, f.Key)
		}

		if !listResult.IsTruncated {
			break
		}
		marker = listResult.NextMarker
	}

	if len(allKeys) == 0 {
		return &BatchDeleteResult{}, nil
	}

	return BatchDelete(ctx, s, allKeys, concurrency), nil
}
