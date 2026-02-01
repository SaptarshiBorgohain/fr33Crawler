import pytest

from knowledge_engine import KnowledgeEngineClient
from knowledge_engine.client import LLMConfig


def test_client_defaults():
    client = KnowledgeEngineClient(api_url="http://example.com/api")
    assert client.api_url == "http://example.com/api"


def test_search_returns_results(monkeypatch):
    client = KnowledgeEngineClient(api_url="http://example.com/api")

    def fake_request_json(path, method="GET", payload=None, headers=None):
        assert path.startswith("/search")
        return {"results": [{"url": "https://example.com", "snippet": "hello", "score": 0.42}]}

    monkeypatch.setattr(client, "_request_json", fake_request_json)
    results = client.search("hello")
    assert len(results) == 1
    assert results[0]["url"] == "https://example.com"


def test_ask_backend(monkeypatch):
    client = KnowledgeEngineClient(api_url="http://example.com/api")

    def fake_request_json(path, method="GET", payload=None, headers=None):
        assert path.startswith("/generate")
        return {"answer": "hello"}

    monkeypatch.setattr(client, "_request_json", fake_request_json)
    assert client.ask_backend("hello") == "hello"


def test_ask_llm_invalid_provider(monkeypatch):
    client = KnowledgeEngineClient(llm_config=LLMConfig(provider="invalid"))
    monkeypatch.setattr(client, "get_search_context", lambda _: [])
    with pytest.raises(ValueError):
        client.ask_llm("hello")
