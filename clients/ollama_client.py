import json
import urllib.request
import urllib.error
from typing import Optional, Generator

from config import OllamaConfig
from clients.protocol import LLMClient


class OllamaClient(LLMClient):
    def __init__(self, cfg: OllamaConfig):
        self._base_url = cfg.base_url.rstrip("/")
        self._ctg_model = cfg.ctg_model
        self._vision_model = cfg.vision_model
        self._timeout = cfg.timeout

    def chat(self, prompt: str, system: str = "", model: str = "", stream: bool = False, reasoning: bool = False, max_tokens: int = 16000) -> str:
        if reasoning:
            return self._chat_with_reasoning(prompt, system, model)

        model_name = model or self._ctg_model
        payload = {
            "model": model_name,
            "prompt": prompt,
            "stream": stream,
        }
        if system:
            payload["system"] = system

        print(f"[LLM] Calling {model_name} (stream={stream}), prompt length: {len(prompt)} chars", flush=True)

        if stream:
            full_response = []
            for token in self._stream_request("/api/generate", payload):
                full_response.append(token)
            result = "".join(full_response)
            print(f"[LLM] Response received: {len(result)} chars", flush=True)
            return result
        else:
            return self._request("/api/generate", payload)

    def chat_stream(self, prompt: str, system: str = "", model: str = "", reasoning: bool = False, max_tokens: int = 16000) -> Generator[str, None, None]:
        """Yield tokens as they arrive from the model."""
        if reasoning:
            return self._stream_with_reasoning(prompt, system, model)

        model_name = model or self._ctg_model
        payload = {
            "model": model_name,
            "prompt": prompt,
            "stream": True,
        }
        if system:
            payload["system"] = system

        return self._stream_request("/api/generate", payload)

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

    def _chat_with_reasoning(self, prompt: str, system: str = "", model: str = "") -> str:
        """Use /api/chat with think=true for reasoning models."""
        model_name = model or self._ctg_model
        messages = [{"role": "user", "content": prompt}]
        if system:
            messages.insert(0, {"role": "system", "content": system})

        payload = {
            "model": model_name,
            "messages": messages,
            "stream": False,
            "options": {"think": True},
        }

        url = f"{self._base_url}/api/chat"
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
                message = result.get("message", {})
                return message.get("content", "").strip()
        except urllib.error.URLError as e:
            raise ConnectionError(f"Ollama reasoning request failed: {e}")

    def _stream_with_reasoning(self, prompt: str, system: str = "", model: str = "") -> Generator[str, None, None]:
        """Use /api/chat with think=true and yield tokens."""
        model_name = model or self._ctg_model
        messages = [{"role": "user", "content": prompt}]
        if system:
            messages.insert(0, {"role": "system", "content": system})

        payload = {
            "model": model_name,
            "messages": messages,
            "stream": True,
            "options": {"think": True},
        }

        url = f"{self._base_url}/api/chat"
        data = json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                for line in resp:
                    if line:
                        chunk = json.loads(line.decode("utf-8"))
                        message = chunk.get("message", {})
                        token = message.get("content", "")
                        if token:
                            yield token
                        if chunk.get("done", False):
                            break
        except urllib.error.URLError as e:
            raise ConnectionError(f"Ollama reasoning stream failed: {e}")

    def is_available(self) -> bool:
        try:
            req = urllib.request.Request(f"{self._base_url}/api/tags")
            urllib.request.urlopen(req, timeout=5)
            return True
        except Exception:
            return False

    def delete_model(self, model_name: str):
        """Delete/unload a model from Ollama memory."""
        try:
            url = f"{self._base_url}/api/delete"
            data = json.dumps({"name": model_name}).encode("utf-8")
            req = urllib.request.Request(
                url,
                data=data,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            urllib.request.urlopen(req, timeout=10)
            print(f"[LLM] Model unloaded: {model_name}", flush=True)
        except Exception as e:
            print(f"[LLM] Failed to unload model {model_name}: {e}", flush=True)

    def _stream_request(self, endpoint: str, payload: dict) -> Generator[str, None, None]:
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
                for line in resp:
                    if line:
                        chunk = json.loads(line.decode("utf-8"))
                        token = chunk.get("response", "")
                        if token:
                            yield token
                        if chunk.get("done", False):
                            break
        except urllib.error.URLError as e:
            raise ConnectionError(f"Ollama stream request failed: {e}")

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
