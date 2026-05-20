# workspace-structure Specification

## Purpose
TBD - created by archiving change storage-revamp. Update Purpose after archive.
## Requirements
### Requirement: Workspace directory structure
The system SHALL create a unified workspace directory for each processed file with the following structure:
```
<workspace>/<file-stem>/
├── chunks/       # Intermediate chunk outputs
├── images/       # Extracted images
├── sections/     # Merged section files
├── intermediate/ # Raw extraction output
└── output/       # Final RAG document
```

#### Scenario: Processing a single file
- **WHEN** a file is processed with workspace `/Users/test/docs` and filename `my-book.pdf`
- **THEN** the directory `/Users/test/docs/my-book/` is created with subdirectories `chunks/`, `images/`, `sections/`, `intermediate/`, `output/`

#### Scenario: File stem sanitization
- **WHEN** the source file has special characters in name `my book (2nd ed.).pdf`
- **THEN** the workspace directory uses sanitized stem `my_book_2nd_ed`

#### Scenario: Workspace path configuration
- **WHEN** the user provides `--workspace /custom/path`
- **THEN** all files are stored under `/custom/path/<file-stem>/`

### Requirement: Workspace path resolution
The system SHALL resolve workspace path in the following priority order:
1. `--workspace` CLI argument
2. `WORKSPACE` environment variable
3. `-o` / `--output` CLI argument (legacy alias)
4. Default: `~/Documents/media2rag/`

#### Scenario: CLI arg takes priority
- **WHEN** `--workspace /a` and `WORKSPACE=/b` are both set
- **THEN** workspace resolves to `/a`

#### Scenario: Legacy -o flag works
- **WHEN** user runs `cli.py file.pdf -o /output`
- **THEN** workspace resolves to `/output`

### Requirement: Chunk storage in workspace
The system SHALL store all chunk-related files in the `chunks/` subdirectory of the workspace:
- `chunk_NNN.md` — processed chunk content
- `metadata.json` — aggregated chunk metadata
- `merged_<section>.md` — intermediate merged sections (before final move to `sections/`)

#### Scenario: Chunks saved to workspace chunks dir
- **WHEN** a large document is split into 5 chunks
- **THEN** files `chunks/chunk_001.md` through `chunks/chunk_005.md` are created

#### Scenario: Resume skips processed chunks
- **WHEN** processing is resumed after 3 of 5 chunks completed
- **THEN** chunks 1-3 are detected as already processed and skipped

