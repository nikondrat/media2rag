## ADDED Requirements

### Requirement: Recursive character split
The Splitter SHALL split text using ordered delimiters: `\n\n\n` → `\n\n` → `\n` → `. ` → hard cut.

#### Scenario: Split at section breaks
- **WHEN** text has multiple blank lines
- **THEN** split occurs at `\n\n\n` boundaries first

#### Scenario: Fallback to sentence boundary
- **WHEN** no paragraph breaks exist
- **THEN** split occurs at `. ` (sentence boundary)

#### Scenario: Hard cut with overlap
- **WHEN** no natural separator found within ChunkSize
- **THEN** hard cut at ChunkSize with configured overlap

### Requirement: Configurable chunk size
The Splitter SHALL accept configurable ChunkSize (default: 4000 chars) and ChunkOverlap (default: 200).

#### Scenario: Custom chunk size
- **WHEN** ChunkSize is set to 8000
- **THEN** chunks are up to 8000 characters
