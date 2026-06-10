## ADDED Requirements

### Requirement: RAG CLI command
The system SHALL provide `media2rag rag <query>` command for hybrid search over indexed chunks.

#### Scenario: Simple search
- **WHEN** `media2rag rag "как масштабировать бизнес"` is called
- **THEN** top 5 most relevant chunks are returned with scores

#### Scenario: Search with filters
- **WHEN** `media2rag rag "метрики продаж" --top 10 --min-score 0.7` is called
- **THEN** up to 10 results with score >= 0.7 are returned

### Requirement: Hybrid search (dense + sparse + RRF)
The RAG command SHALL use hybrid search: dense vector similarity + sparse BM25 + RRF fusion (k=60).

#### Scenario: Dense search
- **WHEN** query embedding is generated
- **THEN** Qdrant returns topK*2 most similar chunks

#### Scenario: Sparse search
- **WHEN** query text is used for BM25
- **THEN** Qdrant returns topK*2 chunks matching keywords

#### Scenario: RRF fusion
- **WHEN** dense and sparse results are merged
- **THEN** RRF score = 1/(60+dense_rank) + 1/(60+sparse_rank), topK returned

### Requirement: JSON output format
The RAG command SHALL support `--format json` for AI agent integration.

#### Scenario: JSON output
- **WHEN** `media2rag rag "query" --format json` is called
- **THEN** output is JSON with results[], each having chunk_id, file, score, topic, summary, key_points

### Requirement: Text output format
The RAG command SHALL support `--format text` (default) for human-readable output.

#### Scenario: Text output
- **WHEN** `media2rag rag "query"` is called
- **THEN** output shows ranked chunks with scores in readable format

### Requirement: Qdrant integration
The RAG command SHALL connect to Qdrant and use indexed chunks.

#### Scenario: Qdrant unavailable
- **WHEN** Qdrant is not running
- **THEN** error "Qdrant not available, run: media2rag index" is returned
