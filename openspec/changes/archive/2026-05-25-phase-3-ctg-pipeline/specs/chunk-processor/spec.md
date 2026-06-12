## ADDED Requirements

### Requirement: Per-chunk LLM processing
The Processor SHALL send each chunk to LLM with a prompt requesting title, topics, and summary.

#### Scenario: KV format response
- **WHEN** LLM processes a chunk
- **THEN** it returns `title: ...\ntopics: ...\nsummary: ...`

#### Scenario: Parse chunk result
- **WHEN** LLM response is parsed
- **THEN** ChunkResult struct is populated with Title, Topics, Summary

### Requirement: Worker pool concurrency
The Processor SHALL process chunks concurrently with configurable max concurrency (default: 3).

#### Scenario: Concurrent processing
- **WHEN** 10 chunks exist and concurrency is 3
- **THEN** at most 3 LLM calls are active simultaneously

#### Scenario: Order preservation
- **WHEN** chunks complete out of order
- **THEN** results are stored in original chunk order
