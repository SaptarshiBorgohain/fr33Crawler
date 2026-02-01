# Knowledge Engine

A production-ready distributed web crawling and indexing system designed for RAG (Retrieval-Augmented Generation) pipelines and AI agent toolchains. This project provides a robust Go backend for high-performance crawling and a Python SDK for seamless integration with inference engines.

## Architecture

The system is architected into two primary components:

### 1. Backend (Go)
The core engine responsible for data acquisition, processing, and indexing.
- **URL Frontier**: Intelligent queue management with priority levels.
- **Politeness Manager**: Enforces domain-specific rates and robots.txt compliance.
- **Vector Search Index**: In-memory TF-IDF vectorizer for semantic retrieval.
- **Content Storage**: JSON-based file persistence for processed data.
- **REST API**: Universal interface for system control and knowledge retrieval.

### 2. SDK (Python)
A lightweight client library for interfacing with the Knowledge Engine.
- **Retrieval Logic**: Automated context fetching for RAG pipelines.
- **Client Interface**: Direct control over crawling and search operations.
- **Integration Layer**: Pre-configured support for local inference servers.

## Getting Started

### Prerequisites
- Go 1.24+
- Python 3.9+
- Local Inference Server (e.g., Ollama)

## Integration Guide (Any Agent / Any Language)

The Knowledge Engine exposes a standard OpenAPI 3.0 interface. You can generate clients in any language or simply use HTTP requests.

### OpenAPI Specification
The full API definition is available in `backend/api/openapi.yaml`. You can feed this file directly to AI Agents (like GPTs or Claude Projects) to enable "Tool Use".

### Quick Integration Examples

**Node.js / TypeScript Agent:**
```typescript
const response = await fetch('http://localhost:8080/api/v1/search?q=my+query');
const context = await response.json();
// Feed `context.results` into your LLM prompt
```

**Curl (Shell):**
```bash
curl -X POST http://localhost:8080/api/v1/crawl -d '{"url":"https://example.com"}'
```

**Docker Deployment (Recommended)**
Run the engine as a sidecar service to your agent:
```bash
docker-compose up -d
```
This launches the Knowledge Engine at `http://localhost:8080`, ready to serve any agent on your network.

### Backend Setup
1. Navigate to the backend directory:
   ```bash
   cd backend
   ```
2. Configure backend model providers (Optional):
   The Knowledge Engine allows you to "hook in" custom LLM providers at the server level.
   ```bash
   # Hooking in a custom OpenAI-compatible model
   export LLM_PROVIDER=openai
   export LLM_BASE_URL=https://api.your-model-host.com/v1/chat/completions
   export LLM_MODEL=your-model-id
   export LLM_API_KEY=your_key
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Start the API service:
   ```bash
   go run cmd/server/main.go
   ```
   The service will be available at `http://localhost:8080`.

### SDK Usage
1. Configure your environment:
   ```bash
   export KNOWLEDGE_ENGINE_API=http://localhost:8080/api/v1
   ```
   *Note: By default, the SDK uses the backend's hooked model. To use client-side generation, set `LLM_PROVIDER` in your shell.*
2. Execute a search or query:
   ```bash
   python sdk/python/client.py ask "What are current best practices in Go concurrency?"
   ```

### Python SDK (pip)
You can install the SDK locally in editable mode:
```bash
cd sdk/python
pip install -e .[dev]
```

Use in your project:
```python
from knowledge_engine import KnowledgeEngineClient

client = KnowledgeEngineClient()
print(client.search("golang memory model"))
```

## Development and Deployment

### Directory Structure
- `backend/`: Core service implementation.
- `sdk/python/`: Client library and demonstration scripts.
- `data/`: Local storage for crawled and indexed documents.

### API Specification
Detailed API documentation and endpoint definitions can be found in `backend/internal/api/server.go`.

## Contributing
Please read `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md` before opening a PR.

## Security
See `SECURITY.md` for reporting vulnerabilities.

## License
This project is released under the MIT License.
