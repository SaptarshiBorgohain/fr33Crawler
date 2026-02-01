package storage_test

import (
	"os"
	"testing"
	// "time" // Removed

	"github.com/stretchr/testify/assert"

	"github.com/knowledge-engine/backend/internal/fetcher"
	"github.com/knowledge-engine/backend/internal/storage"
)

func TestFileStorage(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	fs, err := storage.NewFileStorage(tmpDir)
	assert.NoError(t, err)

	// Test case data
	result := &fetcher.FetchResult{
		URL:        "https://example.com/page1",
		StatusCode: 200,
		Text:       "BodyContent",
	}

	// Test Save
	err = fs.Save(result)
	assert.NoError(t, err)

	// Check file existence
	
	// Test Get
	loaded, err := fs.Get("https://example.com/page1")
	assert.NoError(t, err)
	assert.Equal(t, result.URL, loaded.URL)
	assert.Equal(t, result.Text, loaded.Text)
}

func TestGetNonExistent(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "storage_test_fail")
	defer os.RemoveAll(tmpDir)
	fs, _ := storage.NewFileStorage(tmpDir)

	_, err := fs.Get("https://missing.com")
	assert.Error(t, err)
}
