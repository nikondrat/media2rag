## Why

После CTG Pipeline (Фаза 3) у нас есть RAG-ready документы в workspace. Но чтобы задавать вопросы и получать ответы с источниками, нужен RAG Engine: индексация документов в Qdrant, гибридный поиск (dense + sparse), query rewrite, reranking, parent lookup, dedup. Это превращает систему из "конвертера файлов" в "AI-эксперта с памятью".

## What Changes

- Qdrant client (`internal/store/qdrant.go`) — подключение, инициализация коллекций
- Indexing — создание parent/child точек при сохранении версии
- `internal/rag/` — RAG Engine: query rewrite, hybrid search, rerank, parent lookup, dedup, context build
- `ask` команда — полный цикл: вопрос → RAG → ответ с источниками
- Memory recall — поиск релевантных фактов перед ответом

## Capabilities

### New Capabilities
- `qdrant-store`: Qdrant client, collection management, upsert/search/delete points
- `indexing`: parent/child chunk indexing with content_hash, parent_id payloads
- `rag-engine`: full RAG pipeline (rewrite → search → rerank → dedup → context → LLM)
- `query-rewrite`: format detection, semantic rewrite, multi-query expansion
- `hybrid-search`: dense + sparse search with RRF fusion
- `reranker`: cross-encoder reranking via Ollama /api/rerank
- `parent-lookup`: replace child chunks with parent context
- `context-builder`: build LLM prompt with inline source citations

### Modified Capabilities
- `ask-command`: теперь использует RAG Engine вместо простого LLM chat
- `process-command`: индексирует документ в Qdrant после pipeline

## Impact

- Runtime-зависимость: Qdrant (localhost:6334) должен быть запущен
- Embedding model: nomic-embed-text или аналог
- Reranker model: bge-reranker-v2-m3 (опционально)
- Две коллекции в Qdrant: documents (parent+child) + memories
- Не создаётся: Chat sessions, SQLite, memory store — это фаза 5
