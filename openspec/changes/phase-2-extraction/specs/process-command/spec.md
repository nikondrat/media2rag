## ADDED Requirements

### Requirement: Process command full cycle
The `process` command SHALL execute: detect source → find extractor → extract → save to workspace → emit events.

#### Scenario: Process local markdown
- **WHEN** `media2rag process ./notes.md` is executed
- **THEN** LocalFileExtractor reads file, saves to workspace, emits "completed" event

#### Scenario: Process URL
- **WHEN** `media2rag process "https://example.com"` is executed
- **THEN** URLExtractor calls rdrr, saves result to workspace, emits "completed" event

#### Scenario: Process with --json flag
- **WHEN** `media2rag process ./notes.md --json` is executed
- **THEN** events are emitted as JSON-lines to stdout

#### Scenario: Process without --json flag
- **WHEN** `media2rag process ./notes.md` is executed without --json
- **THEN** human-readable progress is printed to stderr

### Requirement: Process events
The process command SHALL emit events: "extracting", "extracted", "saving", "completed" or "error".

#### Scenario: Extraction started
- **WHEN** extraction begins
- **THEN** event `{"type":"extracting","data":{"source":"..."}}` is emitted

#### Scenario: Extraction completed
- **WHEN** extraction finishes
- **THEN** event `{"type":"extracted","data":{"word_count":1234}}` is emitted

#### Scenario: Error handling
- **WHEN** extraction fails
- **THEN** event `{"type":"error","error":"..."}` is emitted and process exits with code 1

### Requirement: Process flags
The process command SHALL support `--backend`, `--model`, `--workspace`, `--json`, `--extract-only`, `--quality-check`, `--reasoning` flags.

#### Scenario: Custom backend
- **WHEN** `--backend openrouter` is passed
- **THEN** OpenRouter client is used (when pipeline is implemented)

#### Scenario: Extract-only mode
- **WHEN** `--extract-only` is passed
- **THEN** pipeline is skipped, only extraction runs

### Requirement: Unsupported format error
When no extractor matches the source, the command SHALL return a clear error message.

#### Scenario: PDF not supported
- **WHEN** `media2rag process ./doc.pdf` is executed
- **THEN** error "unsupported file format: .pdf (PDF support coming in v2)" is shown
