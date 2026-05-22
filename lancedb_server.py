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

db = None
db_lock = threading.Lock()


def get_db():
    global db
    if db is None:
        os.makedirs(LANCEDB_PATH, exist_ok=True)
        db = lancedb.connect(LANCEDB_PATH)
    return db


def ensure_table(name):
    t = get_db()
    try:
        return t.open_table(name)
    except Exception:
        import pyarrow as pa
        vector_type = pa.list_(pa.float32(), 1024)
        schema = pa.schema([
            pa.field("id", pa.string()),
            pa.field("document_id", pa.string()),
            pa.field("content", pa.string()),
            pa.field("vector", vector_type),
            pa.field("metadata", pa.string()),
        ])
        return t.create_table(name, schema=schema)


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
        else:
            self._send_json({"error": "not found"}, 404)

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/add":
            self._handle_add()
        elif path == "/search":
            self._handle_search()
        elif path == "/create_table":
            self._handle_create_table()
        elif path == "/delete":
            self._handle_delete()
        else:
            self._send_json({"error": "not found"}, 404)

    def _handle_documents(self):
        try:
            with db_lock:
                table = ensure_table("chunks")
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
                    "type": meta.get("type", ""),
                    "section_count": 0,
                }

            for _, row in df.iterrows():
                doc_id = row.get("document_id", "")
                if doc_id in docs:
                    docs[doc_id]["section_count"] += 1

            self._send_json({"documents": list(docs.values())})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

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
        table_name = body.get("table", "chunks")
        records = body.get("records", [])

        if not records:
            self._send_json({"error": "no records"}, 400)
            return

        try:
            import pyarrow as pa
            with db_lock:
                table = ensure_table(table_name)
                data = []
                for r in records:
                    row = {
                        "id": r.get("id", ""),
                        "document_id": r.get("document_id", ""),
                        "content": r.get("content", ""),
                        "vector": [float(v) for v in r.get("embedding", [])],
                        "metadata": json.dumps(r.get("metadata", {})),
                    }
                    data.append(row)

                # Use pyarrow table for proper vector typing
                import pyarrow as pa
                arr = pa.Table.from_pylist(data)
                table.add(data=arr)
            self._send_json({"status": "ok", "added": len(data)})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)

    def _handle_search(self):
        body = self._read_body()
        table_name = body.get("table", "chunks")
        query_vector = body.get("query", [])
        top_k = body.get("topK", 5)
        filter_expr = body.get("filter")

        if not query_vector:
            self._send_json({"error": "no query vector"}, 400)
            return

        try:
            with db_lock:
                table = ensure_table(table_name)
                search = table.search(query_vector, vector_column_name="vector").limit(top_k)
                if filter_expr:
                    search = search.where(filter_expr)
                results = search.to_pandas()

            output = []
            for _, row in results.iterrows():
                output.append({
                    "id": row.get("id", ""),
                    "document_id": row.get("document_id", ""),
                    "content": row.get("content", ""),
                    "metadata": json.loads(row.get("metadata", "{}")),
                    "score": float(row.get("_distance", 0.0)),
                })

            self._send_json({"results": output})
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
            with db_lock:
                table = ensure_table(table_name)
                table.delete(f"document_id = '{escaped}'")
            self._send_json({"status": "ok"})
        except Exception as e:
            self._send_json({"error": str(e)}, 500)


def run_server():
    server = HTTPServer(("127.0.0.1", SERVER_PORT), LanceDBHandler)
    print(f"[lancedb-server] Starting on 127.0.0.1:{SERVER_PORT}", flush=True)
    print(f"[lancedb-server] DB path: {LANCEDB_PATH}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        server.shutdown()
        print("[lancedb-server] Stopped", flush=True)


if __name__ == "__main__":
    run_server()
