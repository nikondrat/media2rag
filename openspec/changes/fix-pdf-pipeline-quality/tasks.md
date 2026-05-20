## 1. Domain Model — content_type field

- [ ] 1.1 Add `content_type: str = "transcript"` field to `ExtractedContent` dataclass in `domain/document.py`
- [ ] 1.2 Add `content_type` to `ExtractedContent` JSON serialization (if any)
- [ ] 1.3 Verify backwards compatibility: existing code that doesn't set `content_type` defaults to `"transcript"`

## 2. Content Type Detection

- [ ] 2.1 Create `detect_content_type(text: str, doc_type: str) -> str` function in new `processors/content_router.py`
- [ ] 2.2 Implement heuristic: transcript = timestamps present OR no headings OR <3 headings
- [ ] 2.3 Implement heuristic: structured-document = ≥3 headings AND ≥5 list items AND no timestamps
- [ ] 2.4 Implement heuristic: mixed = partial structure (1-2 headings with transcript-like content)
- [ ] 2.5 Wire detection into `PdfEpubExtractor.extract()` — set `content_type` on returned `ExtractedContent`
- [ ] 2.6 Wire detection into `VideoExtractor` and `AudioExtractor` — default to `transcript`
- [ ] 2.7 Add unit tests for `detect_content_type()` with sample texts (transcript, structured PDF, mixed)

## 3. CTGPipeline Routing

- [ ] 3.1 Modify `CTGPipeline.process()` to read `extracted.content_type` and branch routing
- [ ] 3.2 Add `transcript` route: existing flow `Compressor.compress() → Transformer.transform() → Generator`
- [ ] 3.3 Add `structured-document` route: `Compressor.compress_structured() → Transformer.transform_structured() → Generator`
- [ ] 3.4 Add `mixed` route: split content, process each part, merge results
- [ ] 3.5 Add fallback: unknown/missing `content_type` → transcript route with warning
- [ ] 3.6 Add JSON-mode emit events for routing decisions (`routing_transcript`, `routing_structured`, `routing_mixed`)

## 4. Structure-Preserving Compressor

- [ ] 4.1 Add `compress_structured(raw_text: str, max_input_tokens: int = 8000) -> str` method to `Compressor`
- [ ] 4.2 Implement OCR artifact cleanup: rejoin hyphenated words across lines
- [ ] 4.3 Implement duplicate header deduplication (consecutive identical headings)
- [ ] 4.4 Implement empty line normalization (3+ → 2)
- [ ] 4.5 Implement structure-aware chunk splitting: split at H1/H2 boundaries, not paragraphs
- [ ] 4.6 Implement section grouping: group consecutive small sections within token limit
- [ ] 4.7 Add compression ratio validation: warn if >70% reduction for structured docs
- [ ] 4.8 Add JSON-mode emit events: `compressing_structured_chunk`, `compressed_structured_chunk`
- [ ] 4.9 Test with Wyckoff PDF chunk_000.md — verify phases A-E are preserved intact

## 5. Structure-Preserving Transformer

- [ ] 5.1 Add `SYSTEM_PROMPT_STRUCTURED` to `Transformer` class
- [ ] 5.2 Prompt: extract metadata (title, author, domains, key_terms, core_thesis) in English via JSON
- [ ] 5.3 Prompt: preserve original body structure, only remove CTA/greetings/self-promotion
- [ ] 5.4 Add `transform_structured(compressed_text: str, existing_metadata) -> tuple[str, DocumentMetadata]` method
- [ ] 5.5 Add `transform_structured_chunk()` for chunked processing (parallel to `transform_chunk()`)
- [ ] 5.6 Ensure body language is preserved (Russian stays Russian, metadata in English)
- [ ] 5.7 Test with Wyckoff compressed output — verify headings and lists are preserved

## 6. Image Analyzer Module

- [ ] 6.1 Create `processors/image_analyzer.py` with `ImageAnalyzer` class
- [ ] 6.2 Implement `__init__(llm_client)` — store client, detect vision capability
- [ ] 6.3 Implement `classify_image(path: str) -> str` — returns diagram/chart/photo/text/icon/decorative
- [ ] 6.4 Classification heuristics: size <100x100 → icon, aspect ratio extreme → decorative
- [ ] 6.5 Implement `analyze(path: str) -> dict | None` — sends to vision LLM, returns description
- [ ] 6.6 Vision prompt: request title, axes, data points, annotations, overall message
- [ ] 6.7 Prompt specifies output language = source document language
- [ ] 6.8 Implement fallback: if no vision capability, return `None` with warning
- [ ] 6.9 Add image filtering: skip <5KB, skip extreme aspect ratios, skip >5MB
- [ ] 6.10 Test with sample Wyckoff diagram image (the Distribution №2 chart)

## 7. Image Integration in Extractor

- [ ] 7.1 Modify `PdfEpubExtractor._extract_with_pymupdf()` to call `ImageAnalyzer` after text extraction
- [ ] 7.2 For each extracted image: classify → analyze (if diagram/chart) → get description
- [ ] 7.3 Insert description into `raw_text` as `> 📊 {description}\n\n![image](path)` at image position
- [ ] 7.4 Update `ExtractedContent.images` entries with `description`, `type`, `analyzed` fields
- [ ] 7.5 Handle EPUB images similarly in `_extract_epub()`
- [ ] 7.6 Add `--skip-images` CLI flag to bypass image analysis
- [ ] 7.7 Test end-to-end: PDF with diagrams → extracted text includes image descriptions

## 8. Hierarchy-Preserving Chunker

- [ ] 8.1 Add `preserve_sections: bool = False` parameter to `SemanticChunker.__init__()`
- [ ] 8.2 Modify `_find_boundaries()` to return only H1/H2 positions when `preserve_sections=True`
- [ ] 8.3 Modify `_split_at_boundaries()` to allow oversized chunks (no split within section)
- [ ] 8.4 Implement related section detection: common prefix + numbering pattern (Фаза А/Б, Phase A/B)
- [ ] 8.5 Implement section grouping: keep related sections together if they fit in TARGET_SIZE
- [ ] 8.6 Modify `_add_overlap()` to apply overlap only at section boundaries when `preserve_sections=True`
- [ ] 8.7 Add `section_name` and `section_level` fields to `Chunk` dataclass
- [ ] 8.8 Add fallback: no headings → paragraph-based splitting regardless of mode
- [ ] 8.9 Test: Wyckoff PDF chunked with `preserve_sections=True` — Phase A-E not split

## 9. ChunkedTransformer Integration

- [ ] 9.1 Pass `preserve_sections=True` to `SemanticChunker` when `content_type` is `structured-document`
- [ ] 9.2 Use `Transformer.transform_structured_chunk()` for structured document chunks
- [ ] 9.3 Ensure `_reduce()` merge respects section boundaries (don't merge across unrelated sections)
- [ ] 9.4 Test map_reduce flow with large structured document (>50K chars)

## 10. CLI Integration

- [ ] 10.1 Add `--skip-images` flag to `cli.py` argument parser
- [ ] 10.2 Pass `skip_images` to extractors
- [ ] 10.3 Add `--preserve-structure` flag to force structured mode (override auto-detection)
- [ ] 10.4 Update JSON-mode output to include `content_type` in `extracted` event
- [ ] 10.5 Test: `uv run cli.py <pdf> --skip-images` works without image analysis
- [ ] 10.6 Test: `uv run cli.py <pdf> --preserve-structure` forces structured mode

## 11. Verification & Testing

- [ ] 11.1 Re-process Wyckoff PDF end-to-end — verify lines 330-454 are no longer hallucinated
- [ ] 11.2 Verify diagram descriptions appear in output markdown
- [ ] 11.3 Verify Phase A-E structure is intact (no fragmented headers)
- [ ] 11.4 Re-process a YouTube transcript — verify no regression (same output as before)
- [ ] 11.5 Test with EPUB book containing diagrams
- [ ] 11.6 Test with mixed content document (part transcript, part structured)
- [ ] 11.7 Verify frontmatter metadata is correct for structured documents
- [ ] 11.8 Run with Ollama backend only (no OpenRouter) — verify all features work
- [ ] 11.9 Run with OpenRouter backend — verify vision model works for images
