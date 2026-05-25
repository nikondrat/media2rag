## 1. Qdrant Store (`internal/store/qdrant.go`)

- [ ] 1.1 Implement Qdrant client connection via gRPC
- [ ] 1.2 Implement `InitCollections()` — create documents + memories collections
- [ ] 1.3 Implement `UpsertPoints()` — insert with vector + payload
- [ ] 1.4 Implement `SearchPoints()` — vector similarity search
- [ ] 1.5 Implement `DeletePoints()` — delete by document_id filter
- [ ] 1.6 Implement `ListCollections()` — list existing collections

## 2. Indexing (`internal/rag/indexing.go`)

- [ ] 2.1 Implement parent chunk splitting (512 tokens)
- [ ] 2.2 Implement child chunk splitting (128 tokens) with parent_id
- [ ] 2.3 Implement content_hash generation (SHA-256)
- [ ] 2.4 Implement embedding generation via Ollama
- [ ] 2.5 Wire indexing into process command after pipeline

## 3. RAG Engine (`internal/rag/engine.go`)

- [ ] 3.1 Implement `RAGEngine` struct with Qdrant store + LLM client
- [ ] 3.2 Implement `Query(ctx, RAGQuery)` — orchestrate full pipeline
- [ ] 3.3 Wire optional stages (rerank) based on config

## 4. Query Rewrite (`internal/rag/rewrite.go`)

- [ ] 4.1 Implement format detection (question/command/statement/fragment)
- [ ] 4.2 Implement semantic rewrite via LLM
- [ ] 4.3 Implement multi-query expansion (3 alternatives)

## 5. Hybrid Search (`internal/rag/search.go`)

- [ ] 5.1 Implement dense vector search via Qdrant
- [ ] 5.2 Implement sparse BM25 search via Qdrant
- [ ] 5.3 Implement RRF fusion with k=60
- [ ] 5.4 Implement topK selection

## 6. Reranker (`internal/rag/rerank.go`)

- [ ] 6.1 Implement cross-encoder rerank via Ollama `/api/rerank`
- [ ] 6.2 Implement scoring and sorting
- [ ] 6.3 Implement topK selection after rerank

## 7. Parent Lookup (`internal/rag/parent.go`)

- [ ] 7.1 Implement parent chunk lookup from child results
- [ ] 7.2 Implement ranking by child match count

## 8. Context Builder (`internal/rag/context.go`)

- [ ] 8.1 Implement source formatting with [N] references
- [ ] 8.2 Implement system prompt with citation instruction
- [ ] 8.3 Implement full context assembly

## 9. Integration

- [ ] 9.1 Wire RAG into `ask` command
- [ ] 9.2 `./media2rag ask "вопрос"` returns answer with sources
- [ ] 9.3 Document indexed in Qdrant after process pipeline
