import json
import urllib.request
import urllib.error


class OllamaEmbedder:
    def __init__(self, base_url: str = "http://localhost:11434", model: str = "qwen3-embedding:0.6b"):
        self._base_url = base_url.rstrip("/")
        self._model = model

    def embed_passages(self, texts: list[str]) -> list[list[float]]:
        return self._embed(texts)

    def embed_query(self, text: str) -> list[float]:
        return self._embed([text])[0]

    def _embed(self, inputs: list[str]) -> list[list[float]]:
        payload = {
            "model": self._model,
            "input": inputs,
        }
        url = f"{self._base_url}/api/embed"
        data = json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                result = json.loads(resp.read().decode("utf-8"))
                return result["embeddings"]
        except urllib.error.URLError as e:
            raise ConnectionError(f"Ollama embed request failed: {e}")

    @property
    def dimensions(self) -> int:
        return 1024
