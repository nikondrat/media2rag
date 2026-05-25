## ADDED Requirements

### Requirement: Unit test coverage
All packages SHALL have unit tests covering core logic.

#### Scenario: Config tests
- **WHEN** `go test ./internal/config/...` runs
- **THEN** tests cover YAML loading, env override, flag merge, validation

#### Scenario: Extractor tests
- **WHEN** `go test ./internal/extract/...` runs
- **THEN** tests cover URL detection, rdrr parsing, local file reading

#### Scenario: Pipeline tests
- **WHEN** `go test ./internal/pipeline/...` runs
- **THEN** tests cover split, assemble, mock LLM processing

#### Scenario: RAG tests
- **WHEN** `go test ./internal/rag/...` runs
- **THEN** tests cover query rewrite, RRF fusion, dedup, context build

### Requirement: Integration tests
Integration tests SHALL test end-to-end flows with real services (skip if unavailable).

#### Scenario: Process integration
- **WHEN** integration test runs with Ollama available
- **THEN** full process pipeline executes successfully

#### Scenario: Skip without services
- **WHEN** Ollama is not running
- **THEN** integration test is skipped
