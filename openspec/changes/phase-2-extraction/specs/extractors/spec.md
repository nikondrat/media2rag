## ADDED Requirements

### Requirement: Extractor interface
The system SHALL define an `Extractor` interface with `Detect(path string) bool` and `Extract(ctx context.Context, path string) (string, error)` methods.

#### Scenario: Interface implementation
- **WHEN** a struct implements both `Detect` and `Extract`
- **THEN** it satisfies the `Extractor` interface

#### Scenario: Extract returns Markdown string
- **WHEN** `Extract` is called on a valid source
- **THEN** it returns Markdown content as string with nil error

### Requirement: Extractor Registry
The system SHALL provide a `Registry` that holds registered extractors and finds the matching one via `Find(path string) (Extractor, error)`.

#### Scenario: Registry finds URL extractor
- **WHEN** `Find("https://youtube.com/watch?v=abc")` is called
- **THEN** it returns `URLExtractor`

#### Scenario: Registry finds local file extractor
- **WHEN** `Find("./notes.md")` is called
- **THEN** it returns `LocalFileExtractor`

#### Scenario: Registry returns error for no match
- **WHEN** `Find("")` is called with empty string
- **THEN** it returns an error

### Requirement: URLExtractor via rdrr
The `URLExtractor` SHALL detect URLs starting with `http://` or `https://` and extract content via `npx rdrr <url> --json`.

#### Scenario: URL detection
- **WHEN** `Detect("https://example.com")` is called
- **THEN** it returns true

#### Scenario: rdrr subprocess execution
- **WHEN** `Extract` is called with a YouTube URL
- **THEN** it executes `npx rdrr <url> --json` and returns the content field

#### Scenario: rdrr JSON parsing
- **WHEN** rdrr returns valid JSON with `content` field
- **THEN** the content is extracted and returned

#### Scenario: rdrr fallback
- **WHEN** rdrr `--json` output is not valid JSON
- **THEN** the full stdout is returned as plain text

#### Scenario: rdrr not available
- **WHEN** `npx rdrr` command is not found
- **THEN** `Extract` returns error "rdrr not found"

### Requirement: LocalFileExtractor
The `LocalFileExtractor` SHALL detect non-URL paths and read `.md`/`.markdown` files natively.

#### Scenario: Local path detection
- **WHEN** `Detect("./notes.md")` is called
- **THEN** it returns true

#### Scenario: Markdown file read
- **WHEN** `Extract` is called on `./notes.md`
- **THEN** it reads the file and returns content (stripping frontmatter)

#### Scenario: Unsupported format
- **WHEN** `Extract` is called on `./doc.pdf`
- **THEN** it returns error "unsupported file format: .pdf"
