## ADDED Requirements

### Requirement: RAG pipeline orchestration
The RAG Engine SHALL execute: query rewrite → hybrid search → rerank (optional) → dedup → context build → LLM answer.

#### Scenario: Full pipeline
- **WHEN** `Query(ctx, RAGQuery)` is called
- **THEN** all stages execute and RAGResponse is returned with answer and sources

#### Scenario: Optional stages
- **WHEN** rerank is disabled in config
- **THEN** rerank stage is skipped

### Requirement: Source citation
The RAG Engine SHALL include source citations in the LLM response.

#### Scenario: Answer with sources
- **WHEN** LLM generates answer
- **THEN** sources are included with [1], [2] references
