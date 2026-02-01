# Agent Context

This project exposes a simple HTTP API for crawling, indexing, and retrieval. AI agents can use this file as a compact “how to use” reference.

## What this service does
- Crawl and index web pages
- Search indexed content with TF‑IDF vectors
- Generate answers with RAG via the backend model

## Core endpoints
Base URL: `http://localhost:8080/api/v1`

- **GET** `/status` — health check
- **POST** `/crawl` — start a crawl
  - Body: `{ "url": "https://example.com" }`
- **GET** `/search?q=<query>` — retrieve ranked snippets
- **GET** `/generate?q=<query>` — RAG answer from backend model

## Typical agent flow
1. **Check service status**
2. **Crawl** a relevant site (if not already indexed)
3. **Search** for a query
4. **Generate** a response using results

## Example request/response (search)
Request:
```
GET /api/v1/search?q=golang%20memory%20model
```

Response (shape):
```json
{
  "results": [
    {
      "url": "https://go.dev/doc/mem",
      "snippet": "...",
      "score": 0.8123
    }
  ]
}
```

## Notes for agents
- Crawling is async; search results may take a few seconds to appear.
- Use `crawl` sparingly and respect the target site’s robots.txt.
- The backend model can be configured via environment variables.

## Python SDK quickstart
```python
from knowledge_engine import KnowledgeEngineClient

client = KnowledgeEngineClient()
client.start_crawl("https://go.dev/")
results = client.search("golang memory model")
answer = client.ask_backend("What is the Go memory model?")
```
