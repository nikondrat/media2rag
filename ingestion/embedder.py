from clients.ollama_embedder import OllamaEmbedder


class Embedder:
    DEFAULT_MODEL = "intfloat/multilingual-e5-small"
    DIMENSIONS = 384

    def __init__(self, model_name: str = DEFAULT_MODEL):
        self._model_name = model_name
        base_url = "http://localhost:11434"
        embed_model = "qwen3-embedding:0.6b"
        self._ollama = OllamaEmbedder(base_url=base_url, model=embed_model)

    def embed_passages(self, texts: list[str]) -> list[list[float]]:
        return self._ollama.embed_passages(texts)

    def embed_query(self, text: str) -> list[float]:
        return self._ollama.embed_query(text)

    @property
    def dimensions(self) -> int:
        return self._ollama.dimensions
