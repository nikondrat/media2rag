## ADDED Requirements

### Requirement: RRF, KeywordOverlapSearch, TopK move to rag package

The system SHALL move search ranking algorithms from `internal/store/qdrant.go` to `internal/rag/ranking.go`.

#### Scenario: RRF is callable from rag package
- **WHEN** `rag/engine.go` needs RRF fusion
- **THEN** it SHALL call `rag.RRF()` instead of `store.RRF()`
- **AND** `store.RRF()` SHALL be removed

#### Scenario: KeywordOverlapSearch is callable from rag package
- **WHEN** `rag/engine.go` needs keyword-based reranking
- **THEN** it SHALL call `rag.KeywordOverlapSearch()` instead of `store.KeywordOverlapSearch()`
- **AND** `store.KeywordOverlapSearch()` SHALL be removed

#### Scenario: TopK is callable from rag package
- **WHEN** any package needs TopK truncation
- **THEN** it SHALL call `rag.TopK()`
- **AND** `store.TopK()` SHALL be removed

### Requirement: store.SearchResult stays in store package

`store.SearchResult` type SHALL remain in `store` package to avoid circular dependencies (rag imports store for results).

#### Scenario: SearchResult is accessible from rag
- **WHEN** `rag/ranking.go` uses SearchResult
- **THEN** it SHALL import `store.SearchResult` — no type duplication

### Requirement: E2E test updated for new import paths

`internal/e2e/rag_test.go` SHALL import ranking functions from `rag` instead of `store`.

#### Scenario: Test compiles after refactor
- **WHEN** running `go test ./internal/e2e/`
- **THEN** all imports SHALL resolve, tests SHALL pass
