## ADDED Requirements

### Requirement: Compressor detects content type and selects compression mode
The Compressor SHALL inspect the `content_type` field of `ExtractedContent` and select the appropriate compression strategy: `compress()` for transcripts, `compress_structured()` for structured documents.

#### Scenario: Transcript content uses standard compression
- **WHEN** `ExtractedContent.content_type` is `transcript`
- **THEN** `Compressor.compress()` is called with the transcript cleanup prompt

#### Scenario: Structured document uses structure-preserving compression
- **WHEN** `ExtractedContent.content_type` is `structured-document`
- **THEN** `Compressor.compress_structured()` is called, which does NOT send content to LLM for rewriting

#### Scenario: Mixed content splits and routes each part
- **WHEN** `ExtractedContent.content_type` is `mixed`
- **THEN** content is split by type boundaries and each part is compressed with the appropriate mode, then reassembled

### Requirement: Structure-preserving compression cleans OCR artifacts without rewriting
The `compress_structured()` method SHALL remove OCR artifacts (broken words, extra line breaks, duplicate headers) while preserving the original document hierarchy (headings, lists, tables) intact.

#### Scenario: Broken words from OCR are rejoined
- **WHEN** text contains words split across lines with hyphens (e.g., "накоп-\nление")
- **THEN** the words are rejoined into single tokens ("накопление")

#### Scenario: Duplicate headers are deduplicated
- **WHEN** the same heading text appears consecutively (e.g., "# Фаза А\n# Фаза А")
- **THEN** only one instance is kept

#### Scenario: Heading hierarchy is preserved
- **WHEN** text contains nested headings (#, ##, ###)
- **THEN** the heading levels and their order are not modified

#### Scenario: Lists and bullet points are preserved
- **WHEN** text contains bullet lists (-, •, *) or numbered lists (1., 2.)
- **THEN** list structure and indentation are preserved without LLM rewriting

#### Scenario: Empty lines are normalized
- **WHEN** text contains 3+ consecutive empty lines
- **THEN** they are reduced to exactly 2 empty lines (one blank line between paragraphs)

### Requirement: Structure-aware chunk splitting for large documents
When `compress_structured()` processes text exceeding `max_input_tokens`, it SHALL split at heading boundaries (H1/H2) rather than paragraph boundaries, ensuring no section is split mid-content.

#### Scenario: Large section becomes its own chunk
- **WHEN** a single section (content under one H2 heading) exceeds `max_input_tokens`
- **THEN** that section is passed as a single chunk even if it exceeds the limit

#### Scenario: Small sections are grouped
- **WHEN** multiple consecutive sections fit within `max_input_tokens`
- **THEN** they are grouped into a single chunk

#### Scenario: Split never occurs mid-list
- **WHEN** a bullet or numbered list spans a potential chunk boundary
- **THEN** the entire list is kept in one chunk

### Requirement: Compression ratio validation
The compressor SHALL track input/output character counts and warn if compression ratio exceeds 70% reduction for structured documents, indicating potential over-compression.

#### Scenario: Normal structured compression passes validation
- **WHEN** `compress_structured()` reduces text by less than 30% (only removing artifacts)
- **THEN** no warning is emitted

#### Scenario: Over-compression triggers warning
- **WHEN** `compress_structured()` reduces text by more than 70%
- **THEN** a warning is logged indicating potential content loss
