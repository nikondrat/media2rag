## 1. Extractors (`internal/extract/`)

- [x] 1.1 Define `Extractor` interface: `Detect(path string) bool`, `Extract(ctx, path) (string, error)`
- [x] 1.2 Implement `Registry` with `Register(extractor)` and `Find(path) (Extractor, error)`
- [x] 1.3 Implement `URLExtractor`: Detect for http/https, Extract via `npx rdrr <url> --json`
- [x] 1.4 Implement rdrr JSON response parsing (title, content, type, metadata)
- [x] 1.5 Implement rdrr fallback for non-JSON output
- [x] 1.6 Implement `LocalFileExtractor`: Detect for non-URL, Extract for `.md`/`.markdown`
- [x] 1.7 Add frontmatter parsing for local markdown files
- [x] 1.8 Return clear error for unsupported formats (pdf, epub, etc.)

## 2. Workspace (`internal/workspace/`)

- [x] 2.1 Define `Workspace` struct with root path
- [x] 2.2 Implement `SourceHash(source string) string` — first 8 chars of SHA-256
- [x] 2.3 Implement `CreateDocument(source string) (*Document, error)` — creates dir structure
- [x] 2.4 Implement `GetDocument(hash string) (*Document, error)` — reads metadata
- [x] 2.5 Implement `ListDocuments() ([]DocumentInfo, error)` — scans workspace dirs
- [x] 2.6 Implement `DeleteDocument(hash string) error` — removes directory
- [x] 2.7 Implement `SaveSource(hash string, markdown string) error` — writes source.md
- [x] 2.8 Implement `SaveVersion(hash string, content string) (int, error)` — writes versions/vN/final.md
- [x] 2.9 Implement `.media2rag.yaml` read/write with metadata struct
- [x] 2.10 Implement `latest` symlink management (with Windows fallback)

## 3. Process Command (`cmd/media2rag/process.go`)

- [x] 3.1 Update process command: parse source arg, load config, create emitter
- [x] 3.2 Wire extractor registry: `Find(source)` → appropriate extractor
- [x] 3.3 Implement extraction flow: emit "extracting" → call Extract → emit "extracted"
- [x] 3.4 Wire workspace: create document → save source.md → emit "saving"
- [x] 3.5 Emit "completed" event with document hash and output path
- [x] 3.6 Handle errors: emit "error" event, exit code 1
- [x] 3.7 Add `--json` flag → StdoutEmitter, without → human-readable stderr output
- [x] 3.8 Add process-specific flags: `--backend`, `--model`, `--extract-only`

## 4. Documents Command (`cmd/media2rag/documents.go`)

- [x] 4.1 Create `documents` parent command with subcommands
- [x] 4.2 Implement `documents list` — calls `workspace.ListDocuments()`, prints table
- [x] 4.3 Implement `documents show <hash>` — reads metadata, displays details
- [x] 4.4 Implement `documents show <hash> --versions` — lists versions
- [x] 4.5 Implement `documents show <hash> --version N` — prints version content
- [x] 4.6 Implement `documents delete <hash>` — with confirmation prompt
- [x] 4.7 Implement `documents delete <hash> --force` — no prompt

## 5. Status Command (`cmd/media2rag/status.go`)

- [x] 5.1 Create `status` command
- [x] 5.2 Implement Ollama health check: GET `/api/tags`
- [x] 5.3 Implement rdrr health check: `npx rdrr --version` or probe
- [x] 5.4 Implement workspace check: directory exists, count documents
- [x] 5.5 Format output: human-readable with ✓/✗ indicators
- [x] 5.6 Add `--json` flag for machine-readable output

## 6. Integration

- [x] 6.1 Register extractors in root command's init
- [x] 6.2 Wire workspace initialization from config
- [x] 6.3 `go build ./cmd/media2rag` succeeds
- [x] 6.4 `./media2rag process ./notes.md` → saves to workspace
- [x] 6.5 `./media2rag process "https://example.com"` → rdrr → saves to workspace
- [x] 6.6 `./media2rag documents list` → shows processed documents
- [x] 6.7 `./media2rag status` → shows service health
