## ADDED Requirements

### Requirement: Embedding quality check
The system SHALL validate embedding quality by comparing Qdrant similarity scores against LLM-judged relevance.

#### Scenario: Embedding check on pipeline completion
- **WHEN** a pipeline run completes
- **THEN** up to `sample_size` (default 5) retrieved chunks are selected for validation

#### Scenario: Similarity scoring
- **WHEN** a chunk is selected for validation
- **THEN** its cosine similarity score from the Qdrant search result is recorded

#### Scenario: LLM relevance check
- **WHEN** a chunk and query are selected
- **THEN** an LLM call evaluates whether the chunk is relevant to the query on a 0.0-1.0 scale

### Requirement: Embedding check storage
The system SHALL store embedding check results in the `embedding_checks` table.

#### Scenario: Check record created
- **WHEN** embedding validation completes for a chunk
- **THEN** an `embedding_checks` record is created with run_id, query_text, chunk_text, similarity_score, relevance_score, passed status, latency_ms, and created_at

#### Scenario: Pass/fail determination
- **WHEN** `relevance_score >= relevance_threshold` (default 0.7) and `similarity_score >= similarity_threshold` (default 0.6)
- **THEN** `passed` is set to 1
- **OTHERWISE** `passed` is set to 0

### Requirement: Aggregate embedding metrics
The system SHALL compute aggregate embedding metrics: avg similarity, avg relevance, pass rate.

#### Scenario: Aggregate computed
- **WHEN** embedding checks exist for a period
- **THEN** the API returns avg similarity, avg relevance, and pass rate across all checks in that period
