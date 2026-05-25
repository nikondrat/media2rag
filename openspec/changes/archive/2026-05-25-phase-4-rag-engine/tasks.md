## 1. Qdrant Store (`internal/store/qdrant.go`)

- [x] 1.1 Implement Qdrant client connection via gRPC
- [x] 1.2 Implement `InitCollections()` — create documents + memories collections
- [x] 1.3 Implement `UpsertPoints()` — insert with vector + payload
- [x] 1.4 Implement `SearchPoints()` — vector similarity search
- [x] 1.5 Implement `DeletePoints()` — delete by document_id filter
- [x] 1.6 Implement `ListCollections()` — list existing collections

## 2. Indexing (`internal/rag/indexing.go`)

- [x] 2.1 Implement parent chunk splitting (512 tokens)
- [x] 2.2 Implement child chunk splitting (128 tokens) with parent_id
- [x] 2.3 Implement content_hash generation (SHA-256)
- [x] 2.4 Implement embedding generation via Ollama
- [x] 2.5 Wire indexing into process command after pipeline

## 3. RAG Engine (`internal/rag/engine.go`)

- [x] 3.1 Implement `RAGEngine` struct with Qdrant store + LLM client
- [x] 3.2 Implement `Query(ctx, RAGQuery)` — orchestrate full pipeline
- [x] 3.3 Wire optional stages (rerank) based on config

## 4. Query Rewrite (`internal/rag/rewrite.go`)

- [x] 4.1 Implement format detection (question/command/statement/fragment)
- [x] 4.2 Implement semantic rewrite via LLM
- [x] 4.3 Implement multi-query expansion (3 alternatives)

## 5. Hybrid Search (`internal/rag/search.go`)

- [x] 5.1 Implement dense vector search via Qdrant
- [x] 5.2 Implement sparse BM25 search via Qdrant
- [x] 5.3 Implement RRF fusion with k=60
- [x] 5.4 Implement topK selection

## 6. Reranker (`internal/rag/rerank.go`)

- [x] 6.1 Implement cross-encoder rerank via Ollama `/api/rerank`
- [x] 6.2 Implement scoring and sorting
- [x] 6.3 Implement topK selection after rerank

## 7. Parent Lookup (`internal/rag/parent.go`)

- [x] 7.1 Implement parent chunk lookup from child results
- [x] 7.2 Implement ranking by child match count

## 8. Context Builder (`internal/rag/context.go`)

- [x] 8.1 Implement source formatting with [N] references
- [x] 8.2 Implement system prompt with citation instruction
- [x] 8.3 Implement full context assembly

## 9. Integration

- [x] 9.1 Wire RAG into `ask` command
- [x] 9.2 `./media2rag ask "вопрос"` returns answer with sources
- [x] 9.3 Document indexed in Qdrant after process pipeline
