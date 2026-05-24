import hashlib
import json

from domain.document import ExtractedContent
from ingestion.normalizer import Normalizer
from ingestion.recursive_chunker import RecursiveChunker
from ingestion.embedder import Embedder
from ingestion.store import LanceDBStore


class IngestionPipeline:
    def __init__(
        self,
        embedder: Embedder | None = None,
        store: LanceDBStore | None = None,
        child_tokens: int = 256,
        parent_tokens: int = 1024,
    ):
        self._normalizer = Normalizer()
        self._chunker = RecursiveChunker(child_tokens=child_tokens, parent_tokens=parent_tokens)
        self._embedder = embedder or Embedder()
        self._store = store or LanceDBStore(dimensions=self._embedder.dimensions)

    def ingest(self, extracted: ExtractedContent, source_path: str = "") -> dict:
        document_id = self._make_document_id(extracted.metadata.source or source_path)

        self._store.delete_document(document_id)

        normalized = self._normalizer.normalize(extracted)

        chunks, parents = self._chunker.chunk(normalized, document_id)

        if not chunks:
            return {"document_id": document_id, "chunks": 0, "parents": 0}

        for c in chunks:
            c.metadata.update(normalized.metadata)
        for p in parents:
            p.metadata.update(normalized.metadata)

        texts = [c.content for c in chunks]
        vectors = self._embedder.embed_passages(texts)

        self._store.store_with_embeddings(chunks, vectors, parents)

        return {
            "document_id": document_id,
            "chunks": len(chunks),
            "parents": len(parents),
            "sections": list({c.section for c in chunks if c.section}),
        }

    @staticmethod
    def _make_document_id(source: str) -> str:
        return hashlib.sha256(source.encode()).hexdigest()[:16]
