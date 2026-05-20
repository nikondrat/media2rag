## ADDED Requirements

### Requirement: Chunk list display
The system SHALL display a list of all chunks in the DetailView when a document was processed in map-reduce mode. Each chunk SHALL show its index, status, word count, and error message if applicable.

#### Scenario: Chunk list for large document
- **WHEN** a document was processed with 12 chunks
- **THEN** the DetailView shows a list of 12 chunk entries with their statuses

#### Scenario: Chunk status indicators
- **WHEN** chunks have various statuses
- **THEN** each chunk shows: ✓ done (green), ⟳ processing (blue), ○ queued (gray), ✗ error (red), ⊘ skipped (orange)

### Requirement: Chunk content preview
The system SHALL allow the user to view the content of any processed chunk by clicking on it. Chunk content SHALL be loaded via memory-mapped I/O from the chunk file.

#### Scenario: Viewing chunk content
- **WHEN** user clicks on a completed chunk
- **THEN** the chunk's Markdown content is displayed in a preview panel

#### Scenario: Chunk file not found
- **WHEN** user clicks on a chunk whose file doesn't exist
- **THEN** an error message is shown: "Chunk file not found"

### Requirement: Chunk retry
The system SHALL allow the user to retry failed chunks individually. A retry SHALL re-run the CLI processing for that specific chunk.

#### Scenario: Retrying a failed chunk
- **WHEN** user clicks "Retry" on a failed chunk
- **THEN** the CLI is invoked to reprocess that specific chunk

### Requirement: Chunk list in QueueItem
The `QueueItem` model SHALL include a `chunks: [ChunkInfo]` array populated from CLI JSON events during map-reduce processing.

#### Scenario: Chunk info populated from events
- **WHEN** CLI emits `map_chunk` events
- **THEN** `QueueItem.chunks` is updated with chunk index, status, and progress
