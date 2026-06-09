## Context

Current `assembler.go` produces a RAG markdown with bare YAML frontmatter (no leading `---`) and `## chunk_NN` sections. `pipeline.go` calls `assemble()` which doesn't exist — only `assembleOutput()` exists with a different signature. The file is further corrupted with duplicated content (lines 227+), preventing compilation.

Output is only written to workspace hash directories (`~/.media2rag/workspace/<hash>/versions/v<N>/final.md`). No structured folder export exists.

Main branch's Python pipeline produced rich folder output: `chunks/`, `intermediate/raw.md`, `output/final.md`, `sections/`, `.media2rag.yaml`. The Go pipeline should replicate this.

## Goals / Non-Goals

**Goals:**
- Fix pipeline.go corruption so binary compiles
- Rewrite assembler to produce format: `---`-delimited YAML frontmatter + `## chunk_NN` sections with knowledge ontology types
- Add `assemble()` function + `assembleOpts` struct connecting pipeline Run to assembler
- Add `--output-dir` flag creating main-branch-compatible folder structure
- Output filename derived from sanitized title

**Non-Goals:**
- No changes to LLM clients, extraction, workspace, or event system
- No section merging (sections/ folder from main branch was Python-specific)
- No changes to the chunk processing pipeline (compressor, splitter, processor)

## Decisions

1. **Leading `---` in frontmatter**: Using standard YAML frontmatter with opening `---` and closing `---` for broader parser compatibility.

2. **Confidence as string in chunk output**: Chunk confidence stored as float internally, rendered as `"high"`/`"medium"`/`"low"` string in output using threshold: >=0.7 → high, >=0.4 → medium, else low.

3. **Folder structure for `--output-dir`**: Mirrors main branch with `chunks/`, `intermediate/raw.md`, `output/<title>.md`, `.media2rag.yaml`. No `sections/` or `images/` (those were Python-specific merging).

4. **Filename sanitization**: Reuse existing `sanitizeFilename()` from assembler.go which replaces `/ \ : * ? " < > |` with safe chars. Preserves original language (Cyrillic, CJK, etc.).

5. **Restore `assemble()` as entry point**: Define `assembleOpts` struct and `assemble()` function in assembler.go that calls `assembleOutput()` internally. This keeps pipeline.go changes minimal.

## Risks / Trade-offs

- **Risk**: Existing workspace consumers expect old format → Mitigation: Workspace stores by version hash; old versions remain. New `process` runs use new format.
- **Risk**: Filename collision from sanitized titles → Mitigation: `sanitizeFilename` already collapses spaces/dashes. If collision occurs, second write overwrites (same behavior as main branch).
- **Trade-off**: Not replicating sections/ merging from main branch — adds complexity with unclear benefit for chunk-based RAG.
