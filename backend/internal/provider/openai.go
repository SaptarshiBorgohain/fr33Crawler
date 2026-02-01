package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type OpenAIProvider struct {
	BaseURL string
	Model   string
	APIKey  string
}

func NewOpenAIProvider(baseURL, model, apiKey string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1/chat/completions"
	}
	return &OpenAIProvider{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Generate(ctx context.Context, prompt string) (string, error) {
	payload := map[string]interface{}{
		"model": p.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai returned status: %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from openai")
	}

	return result.Choices[0].Message.Content, nil
}
