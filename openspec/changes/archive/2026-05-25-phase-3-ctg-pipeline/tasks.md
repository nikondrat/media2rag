## 1. Pipeline Core (`internal/pipeline/`)

- [x] 1.1 Define `Pipeline` struct with stages: Compressor, Splitter, Processor, Assembler
- [x] 1.2 Implement `Run(ctx, rawText, emitter)` — chain stages with event emission
- [x] 1.3 Add error propagation: stop on first stage error
- [x] 1.4 Add PipelineConfig: ChunkSize, ChunkOverlap, MaxConcurrency

## 2. Compressor (`internal/pipeline/compressor.go`)

- [x] 2.1 Implement cleaning prompt: remove timestamps, ads, duplicates, OCR artifacts
- [x] 2.2 Implement LLM-based cleaning via LLMClient
- [x] 2.3 Implement large text chunking: split at paragraphs if > context window
- [x] 2.4 Implement reassembly of cleaned parts
- [x] 2.5 Emit events: compression_start, cleaning_part, compression_done

## 3. Splitter (`internal/pipeline/splitter.go`)

- [x] 3.1 Implement recursive character split with ordered delimiters
- [x] 3.2 Implement overlap between chunks
- [x] 3.3 Handle edge case: text smaller than ChunkSize
- [x] 3.4 Emit events: splitting, split_done

## 4. Chunk Processor (`internal/pipeline/processor.go`)

- [x] 4.1 Implement per-chunk LLM prompt (title + topics + summary)
- [x] 4.2 Implement KV parsing from LLM response
- [x] 4.3 Implement worker pool with configurable concurrency
- [x] 4.4 Preserve chunk order in results
- [x] 4.5 Emit events: processing_start, processing_chunk, processing_chunk_done, processing_done

## 5. Assembler (`internal/pipeline/assembler.go`)

- [x] 5.1 Implement topic merge with deduplication and frequency sorting
- [x] 5.2 Implement title selection (first non-empty or most frequent)
- [x] 5.3 Implement summary combination
- [x] 5.4 Implement YAML frontmatter generation
- [x] 5.5 Generate final Markdown with frontmatter + content
- [x] 5.6 Emit events: assembling, completed

## 6. Integration

- [x] 6.1 Wire pipeline into `process` command after extraction
- [x] 6.2 Save pipeline output to workspace version
- [x] 6.3 `go build ./cmd/media2rag` succeeds
- [x] 6.4 `./media2rag process ./notes.md` runs full pipeline
- [x] 6.5 `./media2rag process "https://example.com"` runs full pipeline

## 7. Pipeline Reliability & Observability

- [x] 7.1 Per-LLM-call timeout via `PipelineConfig.LLMTimeout` + `context.WithTimeout`
- [x] 7.2 Checkpoint resume: compressor saves/loads `compressed.md`
- [x] 7.3 Checkpoint resume: splitter saves/loads `chunks/` dir
- [x] 7.4 Checkpoint resume: processor saves/loads `results.json` (incremental)
- [x] 7.5 Intermediate `.md` artifacts copied to workspace version dir
- [x] 7.6 `--model` flag properly overrides LLM model for pipeline
