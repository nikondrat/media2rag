## ADDED Requirements

### Requirement: Output directory export
The system SHALL support `--output-dir`/`-d` flag on the `process` subcommand that exports the result to a structured directory.

#### Scenario: Output dir flag provided
- **WHEN** user runs `media2rag process --output-dir ./my-output <source>`
- **THEN** system creates `./my-output/` with structured content

### Requirement: Folder structure
The output directory SHALL contain: `chunks/` with individual chunk files, `intermediate/raw.md` with raw extracted content, `output/<title>.md` with final document, `.media2rag.yaml` with processing metadata.

#### Scenario: Folder layout created
- **WHEN** `--output-dir` is specified and processing completes
- **THEN** output directory contains `intermediate/raw.md`, `output/<sanitized-title>.md`, `chunks/chunk_001.md`..., `.media2rag.yaml`

### Requirement: Title-based filename
The filename in `output/` SHALL be derived from the document title via `sanitizeFilename()`, preserving original language while replacing unsafe characters.

#### Scenario: Cyrillic title
- **WHEN** document title is "Как зарабатывать МИЛЛИОНЫ и освободить время для себя? / СЕКРЕТ МИЛЛИАРДЕРА"
- **THEN** output filename SHALL be "Как зарабатывать МИЛЛИОНЫ и освободить время для себя - СЕКРЕТ МИЛЛИАРДЕРА.md"

### Requirement: Chunk files in chunks/
Each processed chunk SHALL be written to `chunks/chunk_NNN.md` with its metadata.

#### Scenario: Chunk written to file
- **WHEN** a chunk has type, topic, summary, and key_points
- **THEN** `chunks/chunk_001.md` contains the chunk's full metadata block

### Requirement: Processing metadata file
The `.media2rag.yaml` SHALL contain: source, source_type, title, model used, word_count, topics, timestamps.

#### Scenario: Metadata file written
- **WHEN** processing completes
- **THEN** `.media2rag.yaml` exists with processing metadata

### Requirement: Chunk confidence as string
Chunk `confidence` field SHALL be rendered as string in output: `high` (≥0.7), `medium` (≥0.4), `low` (<0.4).

#### Scenario: High confidence rendered
- **WHEN** chunk confidence is 0.85
- **THEN** output shows `confidence: high`

#### Scenario: Medium confidence rendered
- **WHEN** chunk confidence is 0.55
- **THEN** output shows `confidence: medium`

#### Scenario: Low confidence rendered
- **WHEN** chunk confidence is 0.2
- **THEN** output shows `confidence: low`

### Requirement: Chunk applicability field
Each chunk SHALL include an `applicability` field with practical usage guidance.

#### Scenario: Applicability present
- **WHEN** chunk has applicability text
- **THEN** output includes `applicability: <text>`
