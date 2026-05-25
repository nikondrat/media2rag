"""Migrate memories from SQLite to LanceDB with embeddings."""

import json
import os
import sqlite3
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

import lancedb
import pyarrow as pa

from clients.ollama_embedder import OllamaEmbedder

SQLITE_PATH = os.path.expanduser("~/Documents/media2rag/chats.sqlite")
LANCEDB_PATH = os.path.expanduser("~/Documents/media2rag/lancedb")


def main():
    embedder = OllamaEmbedder()

    conn = sqlite3.connect(SQLITE_PATH)
    conn.row_factory = sqlite3.Row
    rows = conn.execute("SELECT * FROM memories ORDER BY created_at").fetchall()
    conn.close()

    print(f"Found {len(rows)} memories in SQLite")

    if not rows:
        return

    db = lancedb.connect(LANCEDB_PATH)
    table = _ensure_memories_table(db)

    new_count = 0
    for row in rows:
        memory_id = row["id"]
        content = row["content"]
        category = row["category"]
        confidence = row["confidence"]
        source_session_id = row["source_session_id"] or ""
        created_at = row["created_at"] or ""
        updated_at = row["updated_at"] or ""

        print(f"  [{memory_id[:8]}] {category}: {content[:60]}...")

        vector = embedder.embed_query(content)

        metadata = json.dumps({
            "category": category,
            "confidence": confidence,
            "source_session_id": source_session_id,
            "created_at": created_at,
            "updated_at": updated_at,
        }, ensure_ascii=False)

        data = pa.Table.from_pylist([{
            "id": memory_id,
            "document_id": memory_id,
            "content": content,
            "vector": vector,
            "metadata": metadata,
        }], schema=_schema())

        table.add(data=data)
        new_count += 1

    print(f"\n✅ Added {new_count} memories to LanceDB")


def _ensure_memories_table(db):
    try:
        return db.open_table("memories")
    except Exception:
        return db.create_table("memories", schema=_schema())


def _schema():
    return pa.schema([
        pa.field("id", pa.string()),
        pa.field("document_id", pa.string()),
        pa.field("content", pa.string()),
        pa.field("vector", pa.list_(pa.float32(), 1024)),
        pa.field("metadata", pa.string()),
    ])


if __name__ == "__main__":
    main()
