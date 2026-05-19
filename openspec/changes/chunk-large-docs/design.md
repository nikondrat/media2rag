## Context

The CTG pipeline has a `LARGE_DOC_THRESHOLD` of 50K chars. Documents exceeding this skip compression and transformation entirely — only metadata is extracted from a 15K-char sample taken from the middle of the text. The body is returned raw with only book artifact cleanup.

For a 17K-word Elliott Waves PDF (~90K chars), this means:
- Metadata comes from ~15K chars of what happens to be at the 25% mark
- Body is the full raw text with duplicated table of contents, page separators, and no structure
- No knowledge blocks (Thesis, Mechanism, Framework, etc.)
- Embedding quality suffers from noise-to-signal ratio

## Goals / Non-Goals

**Goals:**
- Every chunk of a large document goes through the full CTG pipeline
- Output is a single unified markdown file with aggregated frontmatter
- Chunking respects natural boundaries (headings, paragraphs) — never mid-sentence
- Processing time scales linearly, not exponentially

**Non-Goals:**
- No parallel chunk processing (sequential to avoid Ollama rate limits)
- No changes to output format — still single .md with frontmatter
- No changes to small document path (< 50K chars)

## Decisions

### Chunking strategy: heading-based with size caps

**Decision:** Split on markdown headings (`##`, `###`) first, fall back to paragraph boundaries, hard cap at 8K chars per chunk.

**Rationale:** Headings are natural semantic boundaries. A section about "Импульс" should stay together. Paragraph-level fallback handles documents without headings. The 8K cap ensures each chunk fits within Ollama context limits after compression.

**Alternatives considered:**
- Fixed-size chunks with overlap: breaks semantic units, poor for knowledge extraction
- LLM-based chunking: too slow and expensive for pre-processing step
- Single-pass with larger sample (30K): still loses structure, wastes context window

### Metadata aggregation strategy

**Decision:** Primary chunk (first substantive one) provides `title`, `core_thesis`, `domains`. All chunks contribute `claims`, `takeaways`, `key_terms` which are deduplicated and merged.

**Rationale:** The first real content section best represents the document's overall thesis. Claims and takeaways accumulate across sections — a 100-page book has more insights than a single chapter can capture.

### Body structure in output

**Decision:** Processed chunks are concatenated in order. Each chunk's structured body (Thesis, Mechanism, etc.) is preserved. A `<!-- chunk: <heading> -->` marker separates sections for traceability.

**Rationale:** Preserves full document structure while keeping each section processed. Markers enable future features like "which chunk did this insight come from?"

## Risks / Trade-offs

**[Risk] Processing time multiplies** → Each chunk requires a full LLM call. A 100-page book might need 8-12 chunks = 8-12 Ollama calls (~5-10 min total).
**Mitigation:** Show progress per chunk, allow `--chunk-size` override for faster processing.

**[Risk] Metadata duplication across chunks** → Same claim extracted from multiple chunks.
**Mitigation:** Deduplicate claims by fuzzy text similarity (80% threshold).

**[Risk] Chunk boundaries lose context** → A concept explained across two chunks gets fragmented.
**Mitigation:** 500-char overlap between adjacent chunks, transformer sees overlap context but deduplicates in output.

**[Risk] Ollama model quality degrades on partial context** → A chunk without the document's introduction may misinterpret references.
**Mitigation:** Include a 200-char "document preamble" (title + first paragraph) in every chunk's system prompt.
