"""Knowledge Engine Python client and CLI helpers."""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any, Callable, Dict, Iterable, List, Optional


@dataclass
class LLMConfig:
    provider: str = os.getenv("LLM_PROVIDER", "ollama").lower()
    model: str = os.getenv("LLM_MODEL", "qwen3:1.7b")
    api_key: str = os.getenv("LLM_API_KEY", "")
    api_base: str = os.getenv(
        "LLM_API_BASE",
        "http://localhost:11434/api/generate"
        if os.getenv("LLM_PROVIDER", "ollama").lower() == "ollama"
        else "https://api.openai.com/v1/chat/completions",
    )


class KnowledgeEngineClient:
    """Thin HTTP client for the Knowledge Engine backend."""

    def __init__(self, api_url: Optional[str] = None, llm_config: Optional[LLMConfig] = None) -> None:
        self.api_url = api_url or os.getenv("KNOWLEDGE_ENGINE_API", "http://localhost:8080/api/v1")
        self.llm_config = llm_config or LLMConfig()

    def _request_json(
        self,
        path: str,
        method: str = "GET",
        payload: Optional[Dict[str, Any]] = None,
        headers: Optional[Dict[str, str]] = None,
    ) -> Dict[str, Any]:
        url = f"{self.api_url}{path}"
        data = None
        if payload is not None:
            data = json.dumps(payload).encode()
        req = urllib.request.Request(url, data=data, headers=headers or {})
        req.method = method

        try:
            with urllib.request.urlopen(req) as response:
                return json.loads(response.read().decode())
        except urllib.error.URLError as e:
            if hasattr(e, "reason") and isinstance(e.reason, ConnectionRefusedError):
                print(f"\n❌ ERROR: Could not connect to Knowledge Engine at {self.api_url}")
                print("   Is the backend running?")
                print("   👉 Try running: 'docker-compose up -d' (No Go required)")
                print("   👉 Or if developing: 'go run cmd/server/main.go'\n")
            raise e

    def check_status(self) -> Optional[Dict[str, Any]]:
        try:
            return self._request_json("/status")
        except Exception:
            return None

    def start_crawl(self, url: str) -> Dict[str, Any]:
        return self._request_json(
            "/crawl",
            method="POST",
            payload={"url": url},
            headers={"Content-Type": "application/json"},
        )

    def search(self, query: str) -> List[Dict[str, Any]]:
        q = urllib.parse.quote(query)
        data = self._request_json(f"/search?q={q}")
        return data.get("results", [])

    def get_search_context(self, query: str) -> List[Dict[str, Any]]:
        return self.search(query)

    def ask_backend(self, query: str) -> str:
        q = urllib.parse.quote(query)
        data = self._request_json(f"/generate?q={q}")
        return data.get("answer", "No answer received.")

    def ask_llm(self, query: str) -> str:
        """Run RAG locally using LLM provider configuration."""
        results = self.get_search_context(query)
        if results:
            context_str = "\n".join([f"- {r.get('snippet', '')}" for r in results])
        else:
            context_str = "No specific context available."

        prompt = (
            "You are a professional AI assistant. Use the provided context to answer the user question.\n"
            "If the context is insufficient, state that clearly but provide the best possible response.\n\n"
            f"CONTEXT:\n{context_str}\n\n"
            f"USER QUESTION:\n{query}\n\nRESPONSE:\n"
        )

        headers = {"Content-Type": "application/json"}
        payload: Dict[str, Any]

        if self.llm_config.provider == "ollama":
            payload = {"model": self.llm_config.model, "prompt": prompt, "stream": False}
        elif self.llm_config.provider == "openai":
            payload = {"model": self.llm_config.model, "messages": [{"role": "user", "content": prompt}]}
            if self.llm_config.api_key:
                headers["Authorization"] = f"Bearer {self.llm_config.api_key}"
        else:
            raise ValueError(f"Unsupported provider '{self.llm_config.provider}'")

        req = urllib.request.Request(
            self.llm_config.api_base,
            data=json.dumps(payload).encode(),
            headers=headers,
        )

        with urllib.request.urlopen(req) as response:
            response_data = json.loads(response.read().decode())

        if self.llm_config.provider == "ollama":
            return response_data.get("response", "No response field.")
        return response_data["choices"][0]["message"]["content"]


def run_cli(argv: Optional[Iterable[str]] = None) -> int:
    args = list(argv or sys.argv[1:])
    client = KnowledgeEngineClient()

    print("Knowledge Engine SDK Interface")
    print("=================================")

    status = client.check_status()
    if not status:
        print("Error: Service unreachable")
        return 1

    if not args:
        if not status.get("running"):
            client.start_crawl("https://go.dev/")
        time.sleep(1)
        print(client.ask_llm("What are the key features of the Go programming language?"))
        return 0

    command = args[0]
    if command == "crawl" and len(args) > 1:
        print(client.start_crawl(args[1]))
        return 0
    if command == "search" and len(args) > 1:
        query = " ".join(args[1:])
        results = client.search(query)
        print(f"Results found: {len(results)}")
        for res in results:
            print(f" - [Score: {res.get('score', 0):.4f}] {res.get('url', '')}")
            print(f"   Snippet: {res.get('snippet', '')[:80]}...")
        return 0
    if command == "ask" and len(args) > 1:
        query = " ".join(args[1:])
        if os.getenv("LLM_PROVIDER"):
            print(client.ask_llm(query))
        else:
            print(client.ask_backend(query))
        return 0

    print("Usage:")
    print("  knowledge-engine crawl <url>")
    print("  knowledge-engine search <query>")
    print("  knowledge-engine ask <query>")
    return 1
