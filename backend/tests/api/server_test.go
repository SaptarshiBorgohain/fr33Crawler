package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/knowledge-engine/backend/internal/api"
	"github.com/knowledge-engine/backend/internal/config"
	"github.com/knowledge-engine/backend/internal/engine"
	"github.com/knowledge-engine/backend/internal/fetcher"
	"github.com/knowledge-engine/backend/internal/search"
)

// Mocks

type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Save(result *fetcher.FetchResult) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *MockStorage) Get(url string) (*fetcher.FetchResult, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fetcher.FetchResult), args.Error(1)
}

func (m *MockStorage) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockLLMProvider struct {
	mock.Mock
}

func (m *MockLLMProvider) Generate(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockLLMProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func setupServer() (*api.Server, *MockLLMProvider, *MockStorage) {
	cfg := config.Load()
	// Disable real crawling for safety in tests if possible, 
	// though StartCrawl just pushes to frontier usually.
	logger := logrus.New().WithField("test", "api")
	store := new(MockStorage)
	vStore := search.NewVectorStore()

	eng, _ := engine.NewEngine(cfg, logger, store, vStore)
	mockLLM := new(MockLLMProvider)
	eng.LLM = mockLLM

	server := api.NewServer(eng, logger)
	return server, mockLLM, store
}

func TestHandleStatus(t *testing.T) {
	server, _, _ := setupServer()

	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	rr := httptest.NewRecorder()

	server.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	
	var resp api.StatusResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.False(t, resp.Running)
}

func TestHandleCrawl(t *testing.T) {
	server, _, _ := setupServer()

	body := strings.NewReader(`{"url": "https://example.com"}`)
	req, _ := http.NewRequest("POST", "/api/v1/crawl", body)
	rr := httptest.NewRecorder()

	server.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestHandleSearch(t *testing.T) {
	server, _, _ := setupServer()
	
	// Add data
	server.Engine.VectorStore.AddDocuments([]*search.Document{
		{ID: "doc1", Content: "Hello world testing search."},
	})

	req, _ := http.NewRequest("GET", "/api/v1/search?q=hello", nil)
	rr := httptest.NewRecorder()

	server.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	
	var resp api.SearchResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "hello", resp.Query)
	// TF-IDF might not have updated if we didn't train or similar, but VectorStore implementation usually does it on fly or simple matching.
	// Checking the implementation of search... it seems to be in-memory vector store.
}

func TestHandleGenerate(t *testing.T) {
	server, mockLLM, _ := setupServer()

	mockLLM.On("Generate", mock.Anything, mock.AnythingOfType("string")).Return("Generated Answer", nil)

	req, _ := http.NewRequest("GET", "/api/v1/generate?q=Anything", nil)
	rr := httptest.NewRecorder()

	server.Router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp api.GenerateResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Generated Answer", resp.Answer)
}
