## MODIFIED Requirements

### Requirement: RAGDocument assembly
The system SHALL define `RAGDocument` struct with `Markdown` and `DocumentMetadata` fields. Assembly SHALL use `assemble(results []ChunkResult, opts assembleOpts) *RAGDocument` calling `assembleOutput(input *AssemblyInput)` internally.

#### Scenario: RAGDocument assembled via assemble()
- **WHEN** pipeline completes
- **THEN** `assemble()` is called with `[]ChunkResult` and `assembleOpts` returning `*RAGDocument`

## ADDED Requirements

### Requirement: assembleOpts struct
The system SHALL define `assembleOpts` struct with fields: source, docType, author, language, domains, coreThesis.

#### Scenario: assembleOpts populated
- **WHEN** pipeline calls `assemble()`
- **THEN** `assembleOpts` contains all pipeline context fields

### Requirement: Frontmatter with opening ---
The output SHALL use standard YAML frontmatter with leading `---`, document-level metadata, closing `---`, and chunk sections.

#### Scenario: Frontmatter formatting
- **WHEN** final document is assembled
- **THEN** output starts with `---`, contains YAML document fields, and ends with `---` before first chunk

### Requirement: DocumentMetadata extra fields
`DocumentMetadata` SHALL additionally contain: ID, Status, MyRelevance, Tags, Confidence (float64).

#### Scenario: Extra metadata populated
- **WHEN** document metadata is assembled
- **THEN** ID, Status, MyRelevance, Tags, Confidence fields are set

### Requirement: AssemblyInput used for output generation
`assembleOutput` SHALL accept `*AssemblyInput` and produce the final markdown string from chunks and metadata.

#### Scenario: AssemblyInput creation
- **WHEN** assembling output
- **THEN** `AssemblyInput` is built from `assembleOpts` and chunk results
