## ADDED Requirements

### Requirement: Parent-child chunk indexing
When a document is processed, the system SHALL create parent chunks (512 tokens) and child chunks (128 tokens) with parent_id references.

#### Scenario: Create parent chunks
- **WHEN** document is processed
- **THEN** parent chunks are created with 512 token size

#### Scenario: Create child chunks
- **WHEN** parent chunks exist
- **THEN** child chunks are created with 128 token size, each referencing parent_id

#### Scenario: Content hash
- **WHEN** chunk is indexed
- **THEN** SHA-256 hash of content is stored as `content_hash` in payload

### Requirement: Embedding generation
Each chunk SHALL be embedded using configured embedding model via Ollama.

#### Scenario: Embed chunk
- **WHEN** chunk is indexed
- **THEN** embedding is generated via Ollama `/api/embed`
