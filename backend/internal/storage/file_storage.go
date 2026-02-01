package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/knowledge-engine/backend/internal/fetcher"
)

// ContentStorage defines the interface for saving crawled data
type ContentStorage interface {
	Save(result *fetcher.FetchResult) error
	Get(url string) (*fetcher.FetchResult, error)
	Close() error
}

// FileStorage implements ContentStorage using the local file system
type FileStorage struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(baseDir string) (*FileStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &FileStorage{
		baseDir: baseDir,
	}, nil
}

// Save writes the fetch result to a JSON file
func (fs *FileStorage) Save(result *fetcher.FetchResult) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Create a safe filename from the URL
	filename := safeFilename(result.URL)
	path := filepath.Join(fs.baseDir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get retrieves a fetch result from disk
func (fs *FileStorage) Get(url string) (*fetcher.FetchResult, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filename := safeFilename(url)
	path := filepath.Join(fs.baseDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var result fetcher.FetchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
}

// Close is a no-op for file storage
func (fs *FileStorage) Close() error {
	return nil
}

// safeFilename converts a URL to a safe filename
func safeFilename(rawURL string) string {
	// A simple hashing or sanitization strategy
	// For readability, let's keep alphanumeric chars and replace others
	safe := ""
	for _, r := range rawURL {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			safe += string(r)
		} else {
			safe += "_"
		}
	}
	// Limit length
	if len(safe) > 100 {
		safe = safe[:100]
	}
	return safe + ".json"
}
