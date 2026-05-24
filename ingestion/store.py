import json
import os
from pathlib import Path

import lancedb
import pyarrow as pa

from domain.chunk import Chunk, ParentChunk


class LanceDBStore:
    DEFAULT_PATH = os.path.expanduser("~/Documents/media2rag/lancedb")
    CHUNKS_TABLE = "chunks_v2"
    PARENTS_TABLE = "parent_chunks_v2"

    def __init__(self, db_path: str = DEFAULT_PATH, dimensions: int = 384):
        self._dimensions = dimensions
        os.makedirs(db_path, exist_ok=True)
        self._db = lancedb.connect(db_path)

    def store(self, chunks: list[Chunk], parents: list[ParentChunk]):
        if chunks:
            self._store_chunks(chunks)
        if parents:
            self._store_parents(parents)

    def _store_chunks(self, chunks: list[Chunk]):
        table = self._get_or_create_chunks_table()
        data = []
        for c in chunks:
            data.append({
                "id": c.id,
                "document_id": c.document_id,
                "parent_id": c.parent_id,
                "content": c.content,
                "vector": [0.0] * self._dimensions,
                "section": c.section,
                "chunk_index": c.chunk_index,
                "content_type": c.content_type,
                "metadata": json.dumps(c.metadata, ensure_ascii=False),
            })
        arr = pa.Table.from_pylist(data, schema=self._chunks_schema())
        table.add(data=arr)

    def store_with_embeddings(self, chunks: list[Chunk], vectors: list[list[float]], parents: list[ParentChunk]):
        if parents:
            self._store_parents(parents)
        if not chunks:
            return

        table = self._get_or_create_chunks_table()
        data = []
        for c, v in zip(chunks, vectors):
            data.append({
                "id": c.id,
                "document_id": c.document_id,
                "parent_id": c.parent_id,
                "content": c.content,
                "vector": v,
                "section": c.section,
                "chunk_index": c.chunk_index,
                "content_type": c.content_type,
                "metadata": json.dumps(c.metadata, ensure_ascii=False),
            })
        arr = pa.Table.from_pylist(data, schema=self._chunks_schema())
        table.add(data=arr)

    def _store_parents(self, parents: list[ParentChunk]):
        table = self._get_or_create_parents_table()
        data = []
        for p in parents:
            data.append({
                "id": p.id,
                "document_id": p.document_id,
                "content": p.content,
                "section": p.section,
                "metadata": json.dumps(p.metadata, ensure_ascii=False),
            })
        arr = pa.Table.from_pylist(data, schema=self._parents_schema())
        table.add(data=arr)

    def get_parent(self, parent_id: str) -> ParentChunk | None:
        try:
            table = self._db.open_table(self.PARENTS_TABLE)
            df = table.to_pandas()
            row = df[df["id"] == parent_id]
            if row.empty:
                return None
            r = row.iloc[0]
            return ParentChunk(
                id=str(r["id"]),
                document_id=str(r["document_id"]),
                content=str(r["content"]),
                section=str(r.get("section", "")),
                metadata=json.loads(r.get("metadata", "{}")),
            )
        except Exception:
            return None

    def delete_document(self, document_id: str):
        escaped = document_id.replace("'", "''")
        filter_expr = f"document_id = '{escaped}'"
        for tname in [self.CHUNKS_TABLE, self.PARENTS_TABLE]:
            try:
                table = self._db.open_table(tname)
                table.delete(filter_expr)
            except Exception:
                pass

    def _get_or_create_chunks_table(self):
        try:
            return self._db.open_table(self.CHUNKS_TABLE)
        except Exception:
            return self._db.create_table(self.CHUNKS_TABLE, schema=self._chunks_schema())

    def _get_or_create_parents_table(self):
        try:
            return self._db.open_table(self.PARENTS_TABLE)
        except Exception:
            return self._db.create_table(self.PARENTS_TABLE, schema=self._parents_schema())

    def _chunks_schema(self) -> pa.Schema:
        return pa.schema([
            pa.field("id", pa.string()),
            pa.field("document_id", pa.string()),
            pa.field("parent_id", pa.string()),
            pa.field("content", pa.string()),
            pa.field("vector", pa.list_(pa.float32(), self._dimensions)),
            pa.field("section", pa.string()),
            pa.field("chunk_index", pa.int32()),
            pa.field("content_type", pa.string()),
            pa.field("metadata", pa.string()),
        ])

    def _parents_schema(self) -> pa.Schema:
        return pa.schema([
            pa.field("id", pa.string()),
            pa.field("document_id", pa.string()),
            pa.field("content", pa.string()),
            pa.field("section", pa.string()),
            pa.field("metadata", pa.string()),
        ])
