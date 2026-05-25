## ADDED Requirements

### Requirement: Pipeline orchestrator
The system SHALL define a `Pipeline` struct that chains stages: Compress → Split → Process → Assemble.

#### Scenario: Pipeline execution
- **WHEN** `Pipeline.Run(ctx, rawText, emitter)` is called
- **THEN** each stage executes in order, passing output to next stage

#### Scenario: Stage error propagation
- **WHEN** a stage returns an error
- **THEN** pipeline stops and returns the error

#### Scenario: Pipeline emits events
- **WHEN** pipeline runs
- **THEN** it emits stage-specific events (compression_start, split_done, etc.)

### Requirement: Compressor stage
The Compressor SHALL clean text by removing timestamps, ads, duplicates, OCR artifacts via LLM.

#### Scenario: LLM cleaning
- **WHEN** `Compressor.Run(ctx, text, emitter)` is called
- **THEN** it sends cleaning prompt to LLM and returns cleaned text

#### Scenario: Large text chunking
- **WHEN** text exceeds context window (>32K tokens)
- **THEN** text is split into parts, each cleaned separately, then reassembled

#### Scenario: Cleaning events
- **WHEN** cleaning a part
- **THEN** event `cleaning_part` with current/total is emitted

### Requirement: Splitter stage
The Splitter SHALL perform recursive character split without LLM, using configurable chunk size and overlap.

#### Scenario: Split by paragraphs
- **WHEN** text contains `\n\n` separators
- **THEN** chunks are split at paragraph boundaries

#### Scenario: Overlap between chunks
- **WHEN** chunk boundary is reached
- **THEN** overlap of configured size is included in next chunk

#### Scenario: No split for small text
- **WHEN** text is smaller than ChunkSize
- **THEN** single chunk is returned

### Requirement: Chunk processor
The Processor SHALL process each chunk via LLM to extract title, topics, summary in KV format.

#### Scenario: Per-chunk processing
- **WHEN** `Processor.Run(ctx, chunks, emitter)` is called
- **THEN** each chunk is sent to LLM with analysis prompt

#### Scenario: Parallel processing
- **WHEN** multiple chunks exist
- **THEN** they are processed concurrently via worker pool (configurable)

#### Scenario: KV parsing
- **WHEN** LLM returns `title: X\ntopics: Y\nsummary: Z`
- **THEN** it is parsed into ChunkResult struct

### Requirement: Assembler stage
The Assembler SHALL merge chunk results into a RAGDocument with YAML frontmatter.

#### Scenario: Merge topics
- **WHEN** chunks have overlapping topics
- **THEN** topics are deduplicated and sorted by frequency

#### Scenario: Generate frontmatter
- **WHEN** document is assembled
- **THEN** YAML frontmatter includes title, topics, summary, word_count

#### Scenario: Markdown output
- **WHEN** document is generated
- **THEN** clean markdown content follows frontmatter
