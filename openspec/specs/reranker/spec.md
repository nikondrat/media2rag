## ADDED Requirements

### Requirement: Cross-encoder reranking
The Reranker SHALL score each chunk against the query using Ollama `/api/rerank` endpoint.

#### Scenario: Rerank chunks
- **WHEN** `Rerank(query, chunks, topK)` is called
- **THEN** each chunk is scored and sorted by relevance

#### Scenario: TopK selection
- **WHEN** 20 chunks are reranked with topK=5
- **THEN** top 5 highest-scoring chunks are returned

### Requirement: Configurable reranker model
The Reranker SHALL use configurable model (default: bge-reranker-v2-m3).

#### Scenario: Custom model
- **WHEN** config specifies different reranker model
- **THEN** that model is used for scoring
