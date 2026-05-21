import json
import time
import urllib.error
import urllib.request
from typing import Optional

from config import OpenRouterConfig


class OpenRouterClient:
    def __init__(self, cfg: OpenRouterConfig):
        self._base_url = cfg.base_url.rstrip("/")
        self._api_key = cfg.api_key
        self._model = cfg.default_model
        self._timeout = cfg.timeout
        self._max_retries = 3

    def chat(self, prompt: str, system: str = "", model: str = "", max_tokens: int = 16000) -> str:
        if not self._api_key:
            raise ValueError("OPENROUTER_API_KEY is not set")

        model_name = model or self._model
        messages = []
        if system:
            messages.append({"role": "system", "content": system})
        messages.append({"role": "user", "content": prompt})

        payload = {
            "model": model_name,
            "messages": messages,
            "stream": False,
            "max_tokens": max_tokens,
        }

        url = f"{self._base_url}/chat/completions"
        data = json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self._api_key}",
                "HTTP-Referer": "https://github.com/nikondrat/media2rag",
                "X-Title": "media2rag",
            },
            method="POST",
        )

        last_error = None
        for attempt in range(self._max_retries):
            try:
                with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                    raw = resp.read().decode("utf-8")
                    result = json.loads(raw)
                    content = result.get("choices", [{}])[0].get("message", {}).get("content")
                    if not content:
                        raise ValueError(f"Empty response from OpenRouter: {result}")
                    return content.strip()
            except urllib.error.HTTPError as e:
                body = e.read().decode("utf-8", errors="replace")
                status = e.code
                if status == 429 and attempt < self._max_retries - 1:
                    wait = 2 ** (attempt + 1)
                    print(f"⚠️  OpenRouter rate limited, retrying in {wait}s (attempt {attempt + 2}/{self._max_retries})")
                    time.sleep(wait)
                    last_error = ConnectionError(f"OpenRouter rate limited (429): {body}")
                    continue
                raise ConnectionError(f"OpenRouter API error {status}: {body}")
            except urllib.error.URLError as e:
                if attempt < self._max_retries - 1:
                    wait = 2 ** (attempt + 1)
                    print(f"⚠️  OpenRouter connection error, retrying in {wait}s (attempt {attempt + 2}/{self._max_retries})")
                    time.sleep(wait)
                    last_error = ConnectionError(f"OpenRouter connection failed: {e.reason}")
                    continue
                raise ConnectionError(f"OpenRouter request failed after {self._max_retries} attempts: {e.reason}")
            except json.JSONDecodeError as e:
                raise ConnectionError(f"OpenRouter returned invalid JSON: {e}")

        raise last_error or ConnectionError("OpenRouter request failed")

    def is_available(self) -> bool:
        return bool(self._api_key)

    def list_models(self) -> list[dict]:
        if not self._api_key:
            raise ValueError("OPENROUTER_API_KEY is not set")

        url = f"{self._base_url}/models"
        req = urllib.request.Request(
            url,
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "HTTP-Referer": "https://github.com/nikondrat/media2rag",
                "X-Title": "media2rag",
            },
            method="GET",
        )

        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                result = json.loads(resp.read().decode("utf-8"))
                models = result.get("data", [])
                return [
                    {
                        "id": m["id"],
                        "name": m.get("name", m["id"]),
                        "description": m.get("description", ""),
                        "context_length": m.get("context_length", 0),
                        "pricing": m.get("pricing", {}),
                    }
                    for m in models
                ]
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            raise ConnectionError(f"OpenRouter API error {e.code}: {body}")

    def validate_key(self) -> bool:
        try:
            self.list_models()
            return True
        except Exception:
            return False
