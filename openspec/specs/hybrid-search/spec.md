## ADDED Requirements

### Requirement: Dense vector search
The system SHALL perform semantic search using embedding similarity in Qdrant.

#### Scenario: Search by embedding
- **WHEN** query embedding is provided
- **THEN** topK*2 most similar points are returned

### Requirement: Sparse BM25 search
The system SHALL perform keyword-based search using Qdrant sparse vectors.

#### Scenario: BM25 search
- **WHEN** query text is provided
- **THEN** topK*2 points matching keywords are returned

### Requirement: RRF fusion
The system SHALL merge dense and sparse results using Reciprocal Rank Fusion with k=60.

#### Scenario: RRF scoring
- **WHEN** dense rank=1, sparse rank=3
- **THEN** score = 1/(60+1) + 1/(60+3)

#### Scenario: TopK selection
- **WHEN** fused results exist
- **THEN** topK highest-scoring results are returned
