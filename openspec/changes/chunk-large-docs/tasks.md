## 1. Chunker Module

- [ ] 1.1 Create `processors/chunker.py` with `DocumentChunk` dataclass (content, heading, index, total, is_primary, preamble)
- [ ] 1.2 Implement heading-based splitting on `##` and `###` boundaries
- [ ] 1.3 Implement paragraph-based fallback split with 8000 char max per chunk
- [ ] 1.4 Implement 500-char overlap between adjacent chunks
- [ ] 1.5 Implement document preamble extraction (title + first 200 chars)
- [ ] 1.6 Implement primary chunk detection (skip TOC, copyright patterns)
- [ ] 1.7 Add unit tests for chunker with sample documents

## 2. Merger Module

- [ ] 2.1 Create `processors/merger.py` with `merge_chunks()` function
- [ ] 2.2 Implement metadata aggregation: primary chunk for title/thesis/domains
- [ ] 2.3 Implement claims aggregation with deduplication (80% text similarity)
- [ ] 2.4 Implement takeaways and key_terms aggregation with deduplication
- [ ] 2.5 Implement body concatenation with `<!-- chunk: <heading> -->` separators
- [ ] 2.6 Implement fallback for single-chunk documents (no markers)

## 3. Pipeline Integration

- [ ] 3.1 Modify `CTGPipeline.process()` large-doc branch to use chunker instead of sample extraction
- [ ] 3.2 Process each chunk through full CTG pipeline (compress → transform)
- [ ] 3.3 Pass document preamble to each chunk's transformer system prompt
- [ ] 3.4 Merge processed chunks using merger module
- [ ] 3.5 Add per-chunk progress emission in JSON mode
- [ ] 3.6 Add `--chunk-size` CLI flag to override 8000 char default

## 4. Testing & Verification

- [ ] 4.1 Test with Elliott Waves PDF — verify structured output for all sections
- [ ] 4.2 Test with long transcript — verify claims/takeaways from multiple sections
- [ ] 4.3 Verify output format matches new frontmatter spec (no topics duplication)
- [ ] 4.4 Verify embedding quality improvement in anythingLLM
