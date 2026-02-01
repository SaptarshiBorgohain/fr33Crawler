# Knowledge Engine Python SDK

This package provides a thin Python client and CLI for the Knowledge Engine backend.

## Features
- Health check and status calls
- Crawl requests
- Search and RAG queries
- Optional LLM calls via Ollama/OpenAI

## Configuration
Environment variables:
- `KNOWLEDGE_ENGINE_API` (default: `http://localhost:8080/api/v1`)
- `LLM_PROVIDER` (`ollama` or `openai`)
- `LLM_MODEL`
- `LLM_API_KEY`
- `LLM_API_BASE`

## Usage
```python
from knowledge_engine import KnowledgeEngineClient

client = KnowledgeEngineClient()
status = client.check_status()
print(status)

client.start_crawl("https://go.dev/")
results = client.search("golang memory model")
print(results)

answer = client.ask_backend("What is the Go memory model?")
print(answer)
```

## CLI
After installation:
- `knowledge-engine crawl <url>`
- `knowledge-engine search <query>`
- `knowledge-engine ask <query>`
