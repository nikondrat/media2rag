## ADDED Requirements

### Requirement: Split document into semantic chunks
The system SHALL split large documents into chunks based on markdown heading boundaries (`##`, `###`), with a maximum chunk size of 8000 characters. When a single section exceeds the limit, it SHALL be further split at paragraph boundaries.

#### Scenario: Heading-based split
- **WHEN** a document contains multiple `##` headings
- **THEN** each heading and its content becomes a separate chunk

#### Scenario: Section exceeds max size
- **WHEN** a single section's content exceeds 8000 characters
- **THEN** the section is split at paragraph boundaries (`\n\n`) into sub-chunks

#### Scenario: No headings in document
- **WHEN** a document has no markdown headings
- **THEN** the document is split at paragraph boundaries into chunks of up to 8000 characters

### Requirement: Include overlap between adjacent chunks
Adjacent chunks SHALL include a 500-character overlap from the preceding chunk's end to preserve context across boundaries.

#### Scenario: Overlap preserves context
- **WHEN** chunk A ends and chunk B begins
- **THEN** chunk B starts with the last 500 characters of chunk A's content

### Requirement: Include document preamble in chunk context
Each chunk SHALL include the document's title and first 200 characters as preamble context for the transformer.

#### Scenario: Preamble included
- **WHEN** a chunk is prepared for transformation
- **THEN** the chunk's context includes the document title and opening paragraph

### Requirement: Identify primary chunk for metadata
The system SHALL designate the first substantive chunk (non-TOC, non-copyright) as the primary chunk for extracting `title`, `core_thesis`, and `domains`.

#### Scenario: Primary chunk selection
- **WHEN** chunks are generated
- **THEN** the first chunk that does not match TOC or copyright patterns is marked as primary

### Requirement: Chunk metadata attachment
Each chunk SHALL carry its heading title, chunk index, total chunk count, and primary flag as metadata.

#### Scenario: Chunk metadata
- **WHEN** a chunk is created
- **THEN** it includes `{heading: str, index: int, total: int, is_primary: bool}`
