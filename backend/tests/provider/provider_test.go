package provider_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/knowledge-engine/backend/internal/provider"
)

type MockTransport struct {
	Response *http.Response
	Err      error
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

func TestOllamaGenerate(t *testing.T) {
	// Mock HTTP Client
	mockResponse := `{"response": "Helsinki is the capital of Finland."}`
	mockTransport := &MockTransport{
		Response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(mockResponse)),
		},
	}
	
	// Swap default transport
	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mockTransport
	defer func() { http.DefaultClient.Transport = oldTransport }()

	p := provider.NewOllamaProvider("http://mock-ollama/api/generate", "llama2")
	
	ans, err := p.Generate(context.Background(), "Capital of Finland?")
	assert.NoError(t, err)
	assert.Equal(t, "Helsinki is the capital of Finland.", ans)
}

func TestOpenAIGenerate(t *testing.T) {
	// Mock HTTP Client
	mockResponse := `{
		"choices": [
			{
				"message": {
					"content": "Paris"
				}
			}
		]
	}`
	mockTransport := &MockTransport{
		Response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(mockResponse)),
		},
	}
	
	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mockTransport
	defer func() { http.DefaultClient.Transport = oldTransport }()

	p := provider.NewOpenAIProvider("http://mock-openai/v1/chat/completions", "gpt-3.5-turbo", "sk-fake")

	ans, err := p.Generate(context.Background(), "Capital of France?")
	assert.NoError(t, err)
	assert.Equal(t, "Paris", ans)
}

func TestProviderFactory(t *testing.T) {
	// Just check if we can create them
	p1 := provider.NewOllamaProvider("", "llama2")
	assert.Equal(t, "ollama", p1.Name())

	p2 := provider.NewOpenAIProvider("", "gpt-4", "key")
	assert.Equal(t, "openai", p2.Name())
}
