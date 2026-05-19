import json
import urllib.request
import urllib.error
from typing import Optional

from config import OllamaConfig


class OllamaClient:
    def __init__(self, cfg: OllamaConfig):
        self._base_url = cfg.base_url.rstrip("/")
        self._ctg_model = cfg.ctg_model
        self._vision_model = cfg.vision_model
        self._timeout = cfg.timeout

    def chat(self, prompt: str, system: str = "", model: str = "") -> str:
        model_name = model or self._ctg_model
        payload = {
            "model": model_name,
            "prompt": prompt,
            "stream": False,
        }
        if system:
            payload["system"] = system

        return self._request("/api/generate", payload)

    def chat_with_image(self, prompt: str, image_b64: str, system: str = "") -> str:
        payload = {
            "model": self._vision_model,
            "prompt": prompt,
            "images": [image_b64],
            "stream": False,
        }
        if system:
            payload["system"] = system

        return self._request("/api/generate", payload)

    def is_available(self) -> bool:
        try:
            req = urllib.request.Request(f"{self._base_url}/api/tags")
            urllib.request.urlopen(req, timeout=5)
            return True
        except Exception:
            return False

    def _request(self, endpoint: str, payload: dict) -> str:
        url = f"{self._base_url}{endpoint}"
        data = json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                result = json.loads(resp.read().decode("utf-8"))
                return result.get("response", "").strip()
        except urllib.error.URLError as e:
            raise ConnectionError(f"Ollama request failed: {e}")
