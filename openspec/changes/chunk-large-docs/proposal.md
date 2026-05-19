## Why

Large documents (PDFs, books, long transcripts) bypass the CTG pipeline entirely. When content exceeds 50K chars, the pipeline only extracts metadata from a 15K-char sample and returns the raw body unchanged — no compression, no structuring, no noise removal. For a 17K-word PDF, this means 2800 lines of unprocessed text with table of contents duplicated, no knowledge blocks, and poor embedding quality.

## What Changes

- Large documents are split into logical chunks before processing
- Each chunk goes through the full CTG pipeline (compress → transform → generate)
- Processed chunks are merged into a single output with unified frontmatter
- Metadata is aggregated from all chunks (thesis from primary, claims/takeaways from all)
- Fallback: if chunking fails, process the first substantial chunk only

## Capabilities

### New Capabilities
- `doc-chunking`: Split large documents into processable segments with overlap context
- `chunk-merge`: Combine processed chunks into unified output with aggregated metadata

### Modified Capabilities
- `ctg-pipeline`: The `process()` method now handles large docs via chunking instead of metadata-only sampling

## Impact

- `processors/ctg_pipeline.py`: Main change — chunking logic replaces the current large-doc branch
- `processors/chunker.py`: New module for document segmentation
- `processors/merger.py`: New module for combining processed chunks
- No breaking changes to CLI interface or output format
