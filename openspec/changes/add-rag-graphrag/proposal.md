## Why

Текущий pipeline обрабатывает документы в Markdown с chunks, causal chains, entities. Но нет CLI команд для поиска по этим знаниям. Vector store (Qdrant) существует но не восстановлен из git. Нет Knowledge Graph — невозможно делать multi-hop queries ("почему X?", "какие топ-5 тем?", "что общего у X и Y?"). AI агенты не могут использовать media2rag как tool.

## What Changes

- **Qdrant restore** — восстановление из git, init collection, indexing chunks
- **Entity & Relation Extraction** — LLM извлекает 12 типов сущностей и 14 типов связей из каждого chunk
- **Graph Storage** — JSON adjacency list (первая фаза), schema для 12 node types + 14 edge types
- **Community Detection** — topic-based clustering, LLM summary для каждого community
- **RAG CLI** — `media2rag rag <query>` — hybrid search (dense + sparse + RRF)
- **GraphRAG CLI** — `media2rag graphrag <query>` — local search (entity fan-out, 2-3 hop traversal) + global search (community summaries)
- **JSON output** — формат для AI агентов с provenance (source_chunk ссылки)

## Capabilities

### New Capabilities
- `graph-extraction`: LLM extraction of 12 entity types + 14 relation types from chunks with deduplication
- `graph-store`: JSON adjacency list storage with node/edge schema, indexing, lookup
- `community-summaries`: topic-based clustering + LLM-generated summaries per community
- `query-rewriter`: LLM preprocessor — natural language → structured query (entities, pattern, mode, depth)
- `rag-cli`: `media2rag rag <query>` — hybrid search with filters, JSON output
- `graphrag-cli`: `media2rag graphrag <query>` — local + global + DRIFT search, multi-hop traversal, JSON output

### Modified Capabilities
- `qdrant-store`: restore from git, collection init, chunk indexing
- `rag-engine`: integrate with graph-store for context building

## Impact

- **15+ файлов изменяются**, 10+ новых файлов создаются
- `internal/graph/`: новый пакет — extraction, store, communities, traversal
- `internal/model/`: новые типы — GraphNode, GraphEdge, Community, GraphQuery
- `cmd/media2rag/`: новые команды — `rag.go`, `graphrag.go`
- Зависимости: Qdrant client (уже есть), embedding model (уже есть)
- Никаких брейкинг-ченджей к существующему pipeline
