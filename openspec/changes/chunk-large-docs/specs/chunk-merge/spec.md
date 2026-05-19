## ADDED Requirements

### Requirement: Merge processed chunks into single document
The system SHALL combine processed chunks into a single markdown document with unified frontmatter.

#### Scenario: Chunks concatenated in order
- **WHEN** all chunks are processed
- **THEN** chunks are concatenated in original order with `<!-- chunk: <heading> -->` separators

### Requirement: Aggregate metadata from all chunks
The system SHALL merge metadata from all processed chunks, using the primary chunk for `title`, `core_thesis`, and `domains`, and combining `claims`, `takeaways`, and `key_terms` from all chunks.

#### Scenario: Primary chunk provides title and thesis
- **WHEN** merging metadata
- **THEN** `title`, `core_thesis`, and `domains` come from the primary chunk's metadata

#### Scenario: Claims aggregated from all chunks
- **WHEN** merging metadata
- **THEN** `claims` from all chunks are combined and deduplicated by text similarity (80% threshold)

#### Scenario: Takeaways aggregated from all chunks
- **WHEN** merging metadata
- **THEN** `takeaways` from all chunks are combined and deduplicated

#### Scenario: Key terms aggregated from all chunks
- **WHEN** merging metadata
- **THEN** `key_terms` from all chunks are combined and deduplicated

### Requirement: Preserve structured body from each chunk
Each chunk's structured body (Thesis, Mechanism, Framework, etc.) SHALL be preserved in the merged output without modification.

#### Scenario: Structured sections preserved
- **WHEN** a chunk's body contains `## Thesis`, `## Mechanism`, etc.
- **THEN** those sections appear in the merged output under the chunk separator

### Requirement: Fallback to single-chunk processing
If chunking produces only one chunk or chunking fails, the system SHALL process the document as a single unit through the standard pipeline.

#### Scenario: Document below threshold
- **WHEN** the document is under 50K characters
- **THEN** the standard (non-chunked) pipeline is used

#### Scenario: Chunking produces single chunk
- **WHEN** chunking yields only one chunk
- **THEN** the document is processed as a single unit without chunk markers
