## Context

После Фазы 3 есть RAG-ready документы в workspace. Фаза 4 добавляет RAG Engine: индексация в Qdrant, поиск, синтез ответов.

**Constraints:**
- Qdrant как external service (localhost:6334)
- Ollama для embeddings (localhost:11434/api/embed)
- Ollama для reranking (localhost:11434/api/rerank, опционально)
- Две коллекции: documents (parent+child) + memories

## Goals / Non-Goals

**Goals:**
- Qdrant client: upsert, search, delete points
- Indexing: parent/child chunks с content_hash, parent_id
- RAG pipeline: rewrite → hybrid search → rerank → dedup → context → LLM
- `ask` команда: вопрос → ответ с источниками
- Memory recall: поиск релевантных фактов

**Non-Goals:**
- Chat sessions, SQLite — фаза 5
- HTTP serve mode — фаза 6
- HyDE, multi-query expansion — v2+

## Decisions

### 1. Qdrant через go-client (gRPC)
**Why:** `github.com/qdrant/go-client` — официальный клиент, gRPC быстрее HTTP.
**Alternatives considered:** HTTP client — проще, но медленнее для bulk operations.

### 2. Parent-child indexing
**Why:** Search по child (точные, 128 tokens), LLM получает parent (контекстные, 512 tokens). Лучшее качество ответа.
**Alternatives considered:** Только parent — теряется точность поиска. Только child — мало контекста для LLM.

### 3. RRF для hybrid search
**Why:** Reciprocal Rank Fusion — стандартная техника, k=60, не требует tuning весов.
**Alternatives considered:** Weighted sum — нужно подбирать веса для каждого домена.

### 4. Reranker опциональный
**Why:** Не у всех есть bge-reranker модель. Reranker включается через config.
**Alternatives considered:** Всегда rerank — лишние LLM вызовы если модель не установлена.

### 5. Query rewrite: format detection без LLM
**Why:** Эвристики (знак вопроса, первое слово) — быстро, бесплатно. Semantic rewrite — 1 LLM вызов.
**Alternatives considered:** Только LLM rewrite — лишний вызов для простых запросов.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Qdrant недоступен | Чёткая ошибка, status check |
| Embedding model mismatch | Configurable model, validation |
| RRF параметры неоптимальны | k=60 — стандарт, configurable |
