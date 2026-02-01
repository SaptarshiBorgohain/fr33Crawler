package engine_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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

func TestNewEngine(t *testing.T) {
	cfg := config.Load()
	logger := logrus.New().WithField("test", "engine")
	store := new(MockStorage)
	vStore := search.NewVectorStore()

	eng, err := engine.NewEngine(cfg, logger, store, vStore)
	assert.NoError(t, err)
	assert.NotNil(t, eng)
	assert.NotNil(t, eng.Frontier)
	assert.NotNil(t, eng.Politeness)
}

func TestEngine_GenerateAnswer(t *testing.T) {
	cfg := config.Load()
	logger := logrus.New().WithField("test", "engine")
	store := new(MockStorage)
	vStore := search.NewVectorStore()

	eng, _ := engine.NewEngine(cfg, logger, store, vStore)
	
	// Add some docs to vector store
	vStore.AddDocuments([]*search.Document{
		{ID: "doc1", Content: "Go is a statically typed, compiled programming language designed at Google."},
		{ID: "doc2", Content: "Concurrency in Go is a first-class citizen."},
	})

	mockLLM := new(MockLLMProvider)
	eng.LLM = mockLLM

	// Expectation
	mockLLM.On("Generate", mock.Anything, mock.AnythingOfType("string")).Return("Go is a language from Google.", nil)

	answer, err := eng.GenerateAnswer(context.Background(), "What is Go?")
	assert.NoError(t, err)
	assert.Equal(t, "Go is a language from Google.", answer)

	mockLLM.AssertExpectations(t)
}

func TestEngine_StartCrawl(t *testing.T) {
	cfg := config.Load()
	logger := logrus.New().WithField("test", "engine")
	store := new(MockStorage)
	vStore := search.NewVectorStore()

	eng, _ := engine.NewEngine(cfg, logger, store, vStore)

	err := eng.StartCrawl("https://example.com")
	assert.NoError(t, err)
	assert.True(t, eng.IsRunning())
}
