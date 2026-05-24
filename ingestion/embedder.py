from functools import lru_cache


class Embedder:
    DEFAULT_MODEL = "intfloat/multilingual-e5-small"
    DIMENSIONS = 384

    def __init__(self, model_name: str = DEFAULT_MODEL):
        self._model_name = model_name
        self._model = None

    def _load(self):
        if self._model is None:
            from sentence_transformers import SentenceTransformer
            self._model = SentenceTransformer(self._model_name)

    def embed_passages(self, texts: list[str]) -> list[list[float]]:
        self._load()
        prefixed = [f"passage: {t}" for t in texts]
        embeddings = self._model.encode(prefixed, normalize_embeddings=True)
        return embeddings.tolist()

    def embed_query(self, text: str) -> list[float]:
        self._load()
        prefixed = f"query: {text}"
        embedding = self._model.encode([prefixed], normalize_embeddings=True)
        return embedding[0].tolist()

    @property
    def dimensions(self) -> int:
        return self.DIMENSIONS
