# Knowledge Engine

**Scalable Web Crawling & Indexing for RAG Pipelines**

Knowledge Engine is a high-performance system that turns the web into a queryable knowledge base for your AI applications. It crawls API-friendly websites, indexes their content specifically for LLMs, and exposes a simple localized search API.

---

## Quick Start (The "Easy" Way)

### 1. Run the Backend Engine

The easiest way to run the engine is using Docker. This starts the crawler and API server instantly.

**Prerequisite:** [Docker Desktop](https://www.docker.com/products/docker-desktop/) installed.

```bash
# Start the service in the background
docker-compose up -d --build
```

The API is now running at `http://localhost:8080`.

**Verify it works:**

```bash
curl http://localhost:8080/api/v1/status
# Output: {"status":"running", ...}
```

---

## Python SDK Integration

We provide a native Python library to easily control the engine and search for answers.

### 1. Install the Package

Run this from the project root:

```bash
pip install ./sdk/python
```

### 2. Usage Example

Save this as `main.py`:

```python
from knowledge_engine import KnowledgeEngineClient

# Connect to your local engine
client = KnowledgeEngineClient("http://localhost:8080/api/v1")

# 1. Teach the engine (Crawl a site)
print("Starting crawl...")
client.start_crawl("https://go.dev/doc/")

# 2. Ask a question (RAG)
# Note: Results appear after crawling finishes (usually 5-10s)
query = "What is the Go memory model?"
print(f"Searching for: {query}")

results = client.search(query)
for res in results['results'][:3]:
    print(f"- Found in {res['url']}: {res['snippet']}")
```

### 3. CLI Tool

The SDK includes a command-line tool for quick testing:

```bash
# Search for something
python -m knowledge_engine search "Go concurrency patterns"

# Crawl a new URL
python -m knowledge_engine crawl "https://example.com"
```

---

## Manual Installation (For Developers)

If you prefer running without Docker, here is how to build from source.

**Prerequisites:**

- **Go** (1.24 or newer)
- **Python** (3.9 or newer)

### 1. Start the Backend

```bash
cd backend
go mod download
go run cmd/server/main.go
```

*Leave this terminal window open.*

### 2. Configure & Run (Advanced)

You can customize the engine using environment variables before running:

```bash
# Example: Change where data is saved and use OpenAI for answers
export SEARCH_DATA_DIR="./my_data"
export LLM_PROVIDER="openai"
export LLM_API_KEY="sk-..."
go run cmd/server/main.go
```

---

## For AI Agents

If you are an autonomous agent (or building one) trying to use this tool, please read **[AGENTS.md](./AGENTS.md)**.
It contains:

- Optimized API usage flows.
- Precise JSON schemas.
- Instructions for tool usage.

---

## Configuration Reference

The system is configured via environment variables.

| Variable                         | Description                          | Default             |
| -------------------------------- | ------------------------------------ | ------------------- |
| `PORT`                         | API Server Port                      | `8080`            |
| `FRONTIER_MAX_CONCURRENCY`     | Max simultaneous crawl jobs          | `10`              |
| `POLITENESS_DEFAULT_MIN_DELAY` | Wait time between requests           | `2s`              |
| `LLM_PROVIDER`                 | AI Provider (`ollama`, `openai`) | `ollama`          |
| `LLM_BASE_URL`                 | URL for the LLM Provider             | (Provider specific) |

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for how to help out!
