## 1. Pipeline Core (`internal/pipeline/`)

- [ ] 1.1 Define `Pipeline` struct with stages: Compressor, Splitter, Processor, Assembler
- [ ] 1.2 Implement `Run(ctx, rawText, emitter)` — chain stages with event emission
- [ ] 1.3 Add error propagation: stop on first stage error
- [ ] 1.4 Add PipelineConfig: ChunkSize, ChunkOverlap, MaxConcurrency

## 2. Compressor (`internal/pipeline/compressor.go`)

- [ ] 2.1 Implement cleaning prompt: remove timestamps, ads, duplicates, OCR artifacts
- [ ] 2.2 Implement LLM-based cleaning via LLMClient
- [ ] 2.3 Implement large text chunking: split at paragraphs if > context window
- [ ] 2.4 Implement reassembly of cleaned parts
- [ ] 2.5 Emit events: compression_start, cleaning_part, compression_done

## 3. Splitter (`internal/pipeline/splitter.go`)

- [ ] 3.1 Implement recursive character split with ordered delimiters
- [ ] 3.2 Implement overlap between chunks
- [ ] 3.3 Handle edge case: text smaller than ChunkSize
- [ ] 3.4 Emit events: splitting, split_done

## 4. Chunk Processor (`internal/pipeline/processor.go`)

- [ ] 4.1 Implement per-chunk LLM prompt (title + topics + summary)
- [ ] 4.2 Implement KV parsing from LLM response
- [ ] 4.3 Implement worker pool with configurable concurrency
- [ ] 4.4 Preserve chunk order in results
- [ ] 4.5 Emit events: processing_start, processing_chunk, processing_chunk_done, processing_done

## 5. Assembler (`internal/pipeline/assembler.go`)

- [ ] 5.1 Implement topic merge with deduplication and frequency sorting
- [ ] 5.2 Implement title selection (first non-empty or most frequent)
- [ ] 5.3 Implement summary combination
- [ ] 5.4 Implement YAML frontmatter generation
- [ ] 5.5 Generate final Markdown with frontmatter + content
- [ ] 5.6 Emit events: assembling, completed

## 6. Integration

- [ ] 6.1 Wire pipeline into `process` command after extraction
- [ ] 6.2 Save pipeline output to workspace version
- [ ] 6.3 `go build ./cmd/media2rag` succeeds
- [ ] 6.4 `./media2rag process ./notes.md` runs full pipeline
- [ ] 6.5 `./media2rag process "https://example.com"` runs full pipeline
