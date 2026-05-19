import json
import urllib.request
import urllib.error
from typing import Optional

from config import OpenRouterConfig


class OpenRouterClient:
    def __init__(self, cfg: OpenRouterConfig):
        self._base_url = cfg.base_url.rstrip("/")
        self._api_key = cfg.api_key
        self._model = cfg.fallback_model
        self._timeout = cfg.timeout

    def chat(self, prompt: str, system: str = "", model: str = "") -> str:
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

        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                result = json.loads(resp.read().decode("utf-8"))
                return result["choices"][0]["message"]["content"].strip()
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            raise ConnectionError(f"OpenRouter API error {e.code}: {body}")

    def is_available(self) -> bool:
        return bool(self._api_key)
