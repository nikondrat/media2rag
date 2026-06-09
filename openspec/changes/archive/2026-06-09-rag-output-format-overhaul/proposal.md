## Why

Current Go pipeline produces a RAG-ready markdown format that's close but doesn't match the user's desired two-layer structure (YAML frontmatter + atomic chunk records). The output lives only in workspace hash-dirs with opaque `final.md` names. There's no `--output-dir` flag for exporting to a structured folder (like the old Python pipeline's output in `main` branch). Additionally, `pipeline.go` is corrupted with duplicated code and won't compile.

## What Changes

- Fix corrupted `pipeline.go` (remove duplicated code, fix `releaseLLM()`)
- Rewrite `assembler.go` to produce matching output format: YAML frontmatter (`---`-delimited) + `## chunk_NN` sections with `type`, `topic`, `summary`, `key_points`, `source_quote`, `my_takeaway`, `confidence`, `applicability`
- Add `assemble()` function + `assembleOpts` to connect pipeline Run → assembler
- Add `--output-dir`/`-d` flag to `process` command
- When `--output-dir` is set, create folder structure matching main branch: `chunks/`, `intermediate/raw.md`, `output/<title>.md`, `.media2rag.yaml`
- Filename in `output/` is sanitized title from frontmatter (original language, unsafe chars replaced)
- Confidence threshold string (e.g. "high"/"medium"/"low") in chunk metadata

## Capabilities

### New Capabilities
- `output-dir-export`: Structured folder export with main-branch-compatible layout, sanitized title filenames

### Modified Capabilities
- `domain-models`: `RAGDocument` assembly now uses new `assemble()` + `assembleOpts` instead of inline assembly; `assembleOutput()` updated for new format
- `output-format-parser`: (unchanged — parser is for LLM output parsing, not document assembly)

## Impact

- `internal/pipeline/assembler.go` — major rewrite
- `internal/pipeline/pipeline.go` — fix corruption, connect to new assembler
- `cmd/media2rag/process.go` — add `--output-dir` flag, folder creation logic
- `cmd/media2rag/main.go` — register new flag variable
