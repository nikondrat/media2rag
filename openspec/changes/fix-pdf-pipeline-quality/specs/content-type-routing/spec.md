## ADDED Requirements

### Requirement: Content type is detected during extraction
The `ExtractedContent` dataclass SHALL include a `content_type: str` field with values `transcript`, `structured-document`, or `mixed`. The extractor SHALL set this field based on content analysis.

#### Scenario: Transcript is detected by absence of structure
- **WHEN** text has no markdown headings and contains timestamps like `[0:00]`
- **THEN** `content_type` is set to `transcript`

#### Scenario: Structured document is detected by heading density
- **WHEN** text has ≥3 markdown headings and ≥5 list items, with no timestamps
- **THEN** `content_type` is set to `structured-document`

#### Scenario: Mixed content is detected by partial structure
- **WHEN** text has some headings (1-2) but also transcript-like content (timestamps, conversational text)
- **THEN** `content_type` is set to `mixed`

#### Scenario: PDF/EPUB defaults to structured-document
- **WHEN** the extractor is `PdfEpubExtractor`
- **THEN** `content_type` defaults to `structured-document` unless transcript patterns are detected

#### Scenario: Video/audio defaults to transcript
- **WHEN** the extractor is `VideoExtractor` or `AudioExtractor`
- **THEN** `content_type` defaults to `transcript`

### Requirement: CTGPipeline routes based on content type
The `CTGPipeline.process()` method SHALL inspect `ExtractedContent.content_type` and route through the appropriate processing strategy.

#### Scenario: Transcript route uses existing pipeline
- **WHEN** `content_type` is `transcript`
- **THEN** `Compressor.compress()` → `Transformer.transform()` → `Generator.generate()` (existing behavior)

#### Scenario: Structured document route uses structure-preserving pipeline
- **WHEN** `content_type` is `structured-document`
- **THEN** `Compressor.compress_structured()` → `Transformer.transform_structured()` → `Generator.generate()`

#### Scenario: Mixed content is split and processed separately
- **WHEN** `content_type` is `mixed`
- **THEN** content is split into transcript and structured parts, each processed with its route, then merged

### Requirement: Transformer has structure-preserving mode
The `Transformer` SHALL provide a `transform_structured()` method that extracts metadata (title, author, domains, key_terms) via LLM but preserves the original document body structure with minimal cleaning.

#### Scenario: Metadata is extracted from structured document
- **WHEN** `transform_structured()` processes a Russian PDF about Wyckoff method
- **THEN** metadata fields (title, author, domains, key_terms, core_thesis) are extracted in English

#### Scenario: Body structure is preserved
- **WHEN** the source has headings "# Базовая схема накопления №1", "## Фаза А", bullet lists
- **THEN** the output body retains these headings and lists in their original form

#### Scenario: Only noise is removed from body
- **WHEN** the source contains CTA text ("подпишитесь", "сохраните этот пост")
- **THEN** such noise is removed from the body

#### Scenario: Original language is preserved in body
- **WHEN** the source document is in Russian
- **THEN** the body content remains in Russian (metadata is in English)

### Requirement: Routing is backwards compatible
When `ExtractedContent.content_type` is not set (None or empty), the pipeline SHALL default to `transcript` mode, preserving existing behavior for all previously working content.

#### Scenario: Missing content_type defaults to transcript
- **WHEN** `ExtractedContent` has no `content_type` field set
- **THEN** the transcript processing route is used

#### Scenario: Unknown content_type defaults to transcript
- **WHEN** `content_type` is set to an unrecognized value
- **THEN** the transcript processing route is used with a warning logged

### Requirement: Content type detection is extensible
The content type detection logic SHALL be implemented as a separate function `detect_content_type(text: str, doc_type: str) -> str` that can be extended with additional heuristics without modifying the pipeline.

#### Scenario: New heuristic can be added
- **WHEN** a new pattern needs to be detected (e.g., table-heavy documents)
- **THEN** a new heuristic can be added to `detect_content_type()` without changing pipeline code
