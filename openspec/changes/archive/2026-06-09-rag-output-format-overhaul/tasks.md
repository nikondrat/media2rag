## 1. Fix pipeline.go corruption

- [x] 1.1 Remove duplicated code (lines 229-471) and fix releaseLLM() body to just `<-p.llmSem`
- [x] 1.2 Verify pipeline.go compiles without errors after fix

## 2. Rewrite assembler.go

- [x] 2.1 Add `assembleOpts` struct with fields: source, docType, author, language, domains, coreThesis
- [x] 2.2 Add `assemble()` function that converts `[]ChunkResult` + `assembleOpts` → `*model.RAGDocument` via `assembleOutput()`
- [x] 2.3 Add leading `---` to frontmatter output
- [x] 2.4 Add `ConfidenceToString()` helper: ≥0.7 → high, ≥0.4 → medium, else low
- [x] 2.5 Update chunk fields to match template (ensure `applicability`, `source_quote`, `my_takeaway` rendered correctly)

## 3. Add --output-dir flag to process command

- [x] 3.1 Add `processOutputDir` variable and register `--output-dir`/`-d` flag in process.go
- [x] 3.2 Implement folder creation: `chunks/`, `intermediate/`, `output/`
- [x] 3.3 Write raw extracted content to `intermediate/raw.md`
- [x] 3.4 Write individual chunk files to `chunks/chunk_NNN.md`
- [x] 3.5 Write final document to `output/<sanitized-title>.md` using `CollectTitle()` + `SanitizeFilename()`
- [x] 3.6 Write `.media2rag.yaml` with processing metadata
- [x] 3.7 Write final.md also to workspace (existing behavior preserved)
- [x] 3.8 Build and verify compilation

## 4. Run on test file

- [x] 4.1 Run `go build ./cmd/media2rag` and verify binary builds
- [x] 4.2 Run processing on `/Volumes/FLASH/grebenukm/КАК открыть бизнес и выйти на прибыль 50 МЛН рублей СОВЕТЫ как БЫСТРО развить дело.md` with `--output-dir`
