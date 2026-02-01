package provider

import (
	"context"
)

// LLMProvider defines the interface for AI model integration
type LLMProvider interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
}

// GenerationRequest defines the internal structure for a RAG request
type GenerationRequest struct {
	Query   string
	Context string
	Model   string
}

func BuildPrompt(query, context string) string {
	if context == "" {
		context = "No specific context available."
	}
	
	return "You are a professional AI assistant. Use the provided context to answer the user question.\n" +
		"If the context is insufficient, state that clearly but provide the best possible response.\n\n" +
		"CONTEXT:\n" + context + "\n\n" +
		"USER QUESTION:\n" + query + "\n\n" +
		"RESPONSE:\n"
}
