#!/usr/bin/env python3
"""LanceDB HTTP bridge server for media2rag-gui Swift client."""

import json
import os
import sys
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import lancedb

LANCEDB_PATH = os.path.expanduser(os.environ.get(
    "LANCEDB_PATH", "~/Documents/media2rag/lancedb"
))
SERVER_PORT = int(os.environ.get("LANCEDB_SERVER_PORT", "54321"))
EMBED_DIMENSIONS = int(os.environ.get("EMBED_DIMENSIONS", "1024"))
EMBED_OLLAMA_MODEL = os.environ.get("EMBED_OLLAMA_MODEL", "qwen3-embedding:0.6b")

db = None
db_lock = threading.Lock()
embedder = None


def get_db():
    global db
    if db is None:
        os.makedirs(LANCEDB_PATH, exist_ok=True)
        db = lancedb.connect(LANCEDB_PATH)
    return db


def get_embedder():
    global embedder
    if embedder is None:
        from clients.ollama_embedder import OllamaEmbedder
        embedder = OllamaEmbedder(model=EMBED_OLLAMA_MODEL)
    return embedder


def embed_texts(texts: list[str]) -> list[list[float]]:
    return get_embedder().embed_passages(texts)


def embed_query(text: str) -> list[float]:
    return get_embedder().embed_query(text)


def ensure_chunks_table():
    import pyarrow as pa
    t = get_db()
    try:
        return t.open_table("chunks_v2")
    except Exception:
        schema = pa.schema([
            pa.field("id", pa.string()),
            pa.field("document_id", pa.string()),
            pa.field("parent_id", pa.string()),
            pa.field("content", pa.string()),
            pa.field("vector", pa.list_(pa.float32(), EMBED_DIMENSIONS)),
            pa.field("section", pa.string()),
            pa.field("chunk_index", pa.int32()),
            pa.field("content_type", pa.string()),
            pa.field("metadata", pa.string()),
        ])
        return t.create_table("chunks_v2", schema=schema)


def ensure_parents_table():
    import pyarrow as pa
    t = get_db()
    try:
        return t.open_table("parent_chunks_v2")
    except Exception:
        schema = pa.schema([
            pa.field("id", pa.string()),
            pa.field("document_id", pa.string()),
            pa.field("content", pa.string()),
            pa.field("section", pa.string()),
            pa.field("metadata", pa.string()),
        ])
        return t.create_table("parent_chunks_v2", schema=schema)


def ensure_memories_table():
    import pyarrow as pa
    t = get_db()
    try:
        return t.open_table("memories")
    except Exception:
        schema = pa.schema([
            pa.field("id", pa.string()),
            pa.field("document_id", pa.string()),
            pa.field("content", pa.string()),
            pa.field("vector", pa.list_(pa.float32(), EMBED_DIMENSIONS)),
            pa.field("metadata", pa.string()),
        ])
        return t.create_table("memories", schema=schema)


def ensure_table(name, with_vector=True):
    if name == "parent_chunks":
        return ensure_parents_table()
    if name == "memories":
        return ensure_memories_table()
    return ensure_chunks_table()


def ensure_fts_index(table):
    try:
        table.list_indices()
    except Exception:
        pass
    try:
        table.create_fts_index("content", replace=True)
        return True
    except Exception:
        return False


def reciprocal_rank_fusion(results_lists: list[list[dict]], k: int = 60) -> list[dict]:
    scores: dict[str, float] = {}
    chunk_map: dict[str, dict] = {}
    best_sim: dict[str, float] = {}

    for results in results_lists:
        for rank, item in enumerate(results):
            cid = item["id"]
            scores[cid] = scores.get(cid, 0.0) + 1.0 / (k + rank + 1)
            if cid not in chunk_map:
                chunk_map[cid] = item
            raw = item.get("_raw_score", 0.0)
            best_sim[cid] = max(best_sim.get(cid, 0.0), raw)

    ranked = sorted(scores.items(), key=lambda x: x[1], reverse=True)
    output = []
    for cid, _ in ranked:
        item = chunk_map[cid].copy()
        item["score"] = best_sim.get(cid, scores[cid])
        output.append(item)
    return output


class LanceDBHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        sys.stderr.write(f"[lancedb-server] {format % args}\n")

    def _send_json(self, data, status=200):
        body = json.dumps(data).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        if length == 0:
            return {}
        return json.loads(self.rfile.read(length))

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/health":
            self._send_json({"status": "ok", "db_path": LANCEDB_PATH})
        elif parsed.path == "/tables":
            with db_lock:
                t = get_db()
                tables = t.table_names()
            self._send_json({"tables": tables})
        elif parsed.path == "/documents":
            self._handle_documents()
        elif parsed.path == "/chunks":
            self._handle_chunks(parsed)
        elif parsed.path == "/chunk":
            self._handle_chunk(parsed)
        elif parsed.path == "/parent":
            self._handle_parent(parsed)
        else:
            self._send_json({"error": "not found"}, 404)

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/add":
            self._handle_add()
        elif path == "/add_parent":
            self._handle_add_parent()
        elif path == "/search":
            self._handle_search()
        elif path == "/bm25_search":
            self._handle_bm25_search()
        elif path == "/hybrid_search":
            self._handle_hybrid_search()
        elif path == "/embed":
            self._handle_embed()
        elif path == "/create_table":
            self._handle_create_table()
        elif path == "/delete":
            self._handle_delete()
        else:
            self._send_json({"error": "not found"}, 404)

    def _handle_documents(self):
        try:
            with db_lock:
                table = ensure_chunks_table()
                df = table.to_pandas()

            docs = {}
            for _, row in df.iterrows():
                doc_id = row.get("document_id", "")
                if not doc_id or doc_id in docs:
                    continue
                meta = json.loads(row.get("metadata", "{}"))
                docs[doc_id] = {
                    "document_id": doc_id,
                    "title": meta.get("title", doc_id),
                    "author": meta.get("author", ""),
                    "source": meta.get("source", ""),
                    "type": meta.get("doc_type", ""),
                    "chunk_count": 0,
                }

            for _, row in df.iterrows():
                doc_id = row.get("document_id", "")
                if doc_id in docs:
                    docs[doc_id]["chunk_count"] += 1

            self._send_json({"documents": list(docs.values())})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_chunks(self, parsed):
        try:
            params = parse_qs(parsed.query)
            document_id = params.get("document_id", [None])[0]

            with db_lock:
                table = ensure_chunks_table()
                df = table.to_pandas()

            if document_id:
                df = df[df["document_id"] == document_id]

            df = df.head(10000)

            chunks = []
            for _, row in df.iterrows():
                chunks.append({
                    "id": row.get("id", ""),
                    "document_id": row.get("document_id", ""),
                    "parent_id": row.get("parent_id", ""),
                    "content": row.get("content", ""),
                    "section": row.get("section", ""),
                    "chunk_index": int(row.get("chunk_index", 0)),
                    "content_type": row.get("content_type", "text"),
                    "metadata": json.loads(row.get("metadata", "{}")),
                })

            self._send_json({"chunks": chunks})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_chunk(self, parsed):
        try:
            params = parse_qs(parsed.query)
            chunk_id = params.get("id", [None])[0]

            if not chunk_id:
                self._send_json({"error": "no id"}, 400)
                return

            with db_lock:
                table = get_db().open_table("chunks_v2")
                df = table.to_pandas()
                row = df[df["id"] == chunk_id]

            if row.empty:
                self._send_json({"error": "not found"}, 404)
                return

            r = row.iloc[0]
            result = {
                "id": str(r.get("id", "")),
                "document_id": str(r.get("document_id", "")),
                "parent_id": str(r.get("parent_id", "")),
                "content": str(r.get("content", "")),
                "section": str(r.get("section", "")),
                "metadata": json.loads(r.get("metadata", "{}")),
            }

            parent_id = result["parent_id"]
            if parent_id:
                parent = self._lookup_parent(parent_id)
                if parent:
                    result["parent_content"] = parent["content"]
                    result["parent_section"] = parent.get("section", "")

            self._send_json({"chunk": result})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_parent(self, parsed):
        try:
            params = parse_qs(parsed.query)
            parent_id = params.get("id", [None])[0]

            if not parent_id:
                self._send_json({"error": "no id"}, 400)
                return

            parent = self._lookup_parent(parent_id)
            if not parent:
                self._send_json({"error": "not found"}, 404)
                return

            self._send_json({"parent": parent})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _lookup_parent(self, parent_id: str) -> dict | None:
        try:
            with db_lock:
                table = get_db().open_table("parent_chunks_v2")
                df = table.to_pandas()
                row = df[df["id"] == parent_id]
            if row.empty:
                return None
            r = row.iloc[0]
            return {
                "id": str(r.get("id", "")),
                "document_id": str(r.get("document_id", "")),
                "content": str(r.get("content", "")),
                "section": str(r.get("section", "")),
                "metadata": json.loads(r.get("metadata", "{}")),
            }
        except Exception:
            return None

    def _handle_create_table(self):
        body = self._read_body()
        table_name = body.get("table", "chunks")
        try:
            with db_lock:
                ensure_table(table_name)
            self._send_json({"status": "ok", "table": table_name})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_add(self):
        body = self._read_body()
        records = body.get("records", [])
        table_name = body.get("table", "chunks_v2")

        if not records:
            self._send_json({"error": "no records"}, 400)
            return

        try:
            import pyarrow as pa
            with db_lock:
                table = ensure_table(table_name, with_vector=True)
                data = []
                for r in records:
                    row = {
                        "id": r.get("id", ""),
                        "document_id": r.get("document_id", r.get("id", "")),
                        "content": r.get("content", ""),
                        "vector": [float(v) for v in r.get("embedding", [0.0] * EMBED_DIMENSIONS)],
                        "metadata": json.dumps(r.get("metadata", {})),
                    }
                    if "parent_id" in r:
                        row["parent_id"] = r["parent_id"]
                    if "section" in r:
                        row["section"] = r["section"]
                    if "chunk_index" in r:
                        row["chunk_index"] = r["chunk_index"]
                    if "content_type" in r:
                        row["content_type"] = r["content_type"]
                    data.append(row)

                arr = pa.Table.from_pylist(data)
                table.add(data=arr)

            self._send_json({"status": "ok", "added": len(data)})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_add_parent(self):
        body = self._read_body()
        records = body.get("records", [])

        if not records:
            self._send_json({"error": "no records"}, 400)
            return

        try:
            import pyarrow as pa
            with db_lock:
                table = ensure_parents_table()
                data = []
                for r in records:
                    row = {
                        "id": r.get("id", ""),
                        "document_id": r.get("document_id", ""),
                        "content": r.get("content", ""),
                        "section": r.get("section", ""),
                        "metadata": json.dumps(r.get("metadata", {})),
                    }
                    data.append(row)

                arr = pa.Table.from_pylist(data)
                table.add(data=arr)

            self._send_json({"status": "ok", "added": len(data)})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_embed(self):
        body = self._read_body()
        texts = body.get("texts", [])
        query = body.get("query", "")

        if not texts and not query:
            self._send_json({"error": "no texts or query"}, 400)
            return

        try:
            if query:
                vector = embed_query(query)
                self._send_json({"embedding": vector, "dimensions": len(vector)})
            else:
                vectors = embed_texts(texts)
                self._send_json({"embeddings": vectors, "dimensions": len(vectors[0]) if vectors else 0})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_search(self):
        body = self._read_body()
        query_vector = body.get("query", [])
        top_k = body.get("topK", 5)
        filter_expr = body.get("filter")
        table_name = body.get("table", "chunks_v2")

        if not query_vector:
            self._send_json({"error": "no query vector"}, 400)
            return

        try:
            with db_lock:
                table = ensure_table(table_name, with_vector=True)
                search = table.search(query_vector, vector_column_name="vector").limit(top_k)
                if filter_expr:
                    search = search.where(filter_expr)
                results = search.to_pandas()

            output = []
            for _, row in results.iterrows():
                dist = float(row.get("_distance", 0.0))
                output.append({
                    "id": row.get("id", ""),
                    "document_id": row.get("document_id", ""),
                    "content": row.get("content", ""),
                    "metadata": json.loads(row.get("metadata", "{}")),
                    "score": max(0.0, 1.0 - dist),
                })

            self._send_json({"results": output})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_bm25_search(self):
        body = self._read_body()
        query_text = body.get("query", "")
        top_k = body.get("topK", 5)
        filter_expr = body.get("filter")

        if not query_text:
            self._send_json({"error": "no query text"}, 400)
            return

        try:
            with db_lock:
                table = ensure_chunks_table()
                ensure_fts_index(table)

                search = table.search(query_text, query_type="fts").limit(top_k)
                if filter_expr:
                    search = search.where(filter_expr)
                results = search.to_pandas()

            output = []
            for _, row in results.iterrows():
                output.append({
                    "id": row.get("id", ""),
                    "document_id": row.get("document_id", ""),
                    "parent_id": row.get("parent_id", ""),
                    "content": row.get("content", ""),
                    "section": row.get("section", ""),
                    "metadata": json.loads(row.get("metadata", "{}")),
                    "score": float(row.get("_score", 0.0)),
                })

            self._send_json({"results": output})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_hybrid_search(self):
        body = self._read_body()
        query_vector = body.get("query_vector", [])
        query_text = body.get("query_text", "")
        top_k = body.get("topK", 5)
        filter_expr = body.get("filter")
        include_parent = body.get("include_parent", False)

        if not query_vector and not query_text:
            self._send_json({"error": "no query"}, 400)
            return

        try:
            if not query_vector and query_text:
                query_vector = embed_query(query_text)

            with db_lock:
                table = ensure_chunks_table()

            results_lists = []

            if query_vector:
                with db_lock:
                    search = table.search(query_vector, vector_column_name="vector").limit(top_k * 2)
                    if filter_expr:
                        search = search.where(filter_expr)
                    vector_results = search.to_pandas()

                vector_output = []
                for _, row in vector_results.iterrows():
                    dist = float(row.get("_distance", 0.0))
                    vector_output.append({
                        "id": row.get("id", ""),
                        "document_id": row.get("document_id", ""),
                        "parent_id": row.get("parent_id", ""),
                        "content": row.get("content", ""),
                        "section": row.get("section", ""),
                        "metadata": json.loads(row.get("metadata", "{}")),
                        "score": dist,
                        "_raw_score": max(0.0, 1.0 - dist),
                    })
                results_lists.append(vector_output)

            if query_text:
                with db_lock:
                    ensure_fts_index(table)
                    search = table.search(query_text, query_type="fts").limit(top_k * 2)
                    if filter_expr:
                        search = search.where(filter_expr)
                    fts_results = search.to_pandas()

                fts_output = []
                for _, row in fts_results.iterrows():
                    fts_score = float(row.get("_score", 0.0))
                    fts_output.append({
                        "id": row.get("id", ""),
                        "document_id": row.get("document_id", ""),
                        "parent_id": row.get("parent_id", ""),
                        "content": row.get("content", ""),
                        "section": row.get("section", ""),
                        "metadata": json.loads(row.get("metadata", "{}")),
                        "score": fts_score,
                        "_raw_score": 0.0,
                    })
                if fts_output:
                    max_fs = max(r["score"] for r in fts_output) or 1.0
                    for r in fts_output:
                        r["_raw_score"] = r["score"] / max_fs
                results_lists.append(fts_output)

            fused = reciprocal_rank_fusion(results_lists)[:top_k]

            if include_parent:
                for item in fused:
                    parent_id = item.get("parent_id", "")
                    if parent_id:
                        parent = self._lookup_parent(parent_id)
                        if parent:
                            item["parent_content"] = parent["content"]
                            item["parent_section"] = parent.get("section", "")

            self._send_json({"results": fused})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_delete(self):
        body = self._read_body()
        table_name = body.get("table", "chunks")
        document_id = body.get("document_id")

        if not document_id:
            self._send_json({"error": "no document_id"}, 400)
            return

        try:
            escaped = document_id.replace("'", "''")
            filter_expr = f"document_id = '{escaped}'"
            with db_lock:
                for tname in ["chunks_v2", "parent_chunks_v2"]:
                    try:
                        table = get_db().open_table(tname)
                        table.delete(filter_expr)
                    except Exception:
                        pass
            self._send_json({"status": "ok"})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)


def run_server():
    server = HTTPServer(("127.0.0.1", SERVER_PORT), LanceDBHandler)
    print(f"[lancedb-server] Starting on 127.0.0.1:{SERVER_PORT}", flush=True)
    print(f"[lancedb-server] DB path: {LANCEDB_PATH}", flush=True)
    print(f"[lancedb-server] Embed dimensions: {EMBED_DIMENSIONS}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        server.shutdown()
        print("[lancedb-server] Stopped", flush=True)


if __name__ == "__main__":
    run_server()
