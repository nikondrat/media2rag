## ADDED Requirements

### Requirement: model/types.go is split by domain

The system SHALL split `internal/model/types.go` into multiple files by bounded context, all within `package model`.

#### Scenario: Each domain has its own file
- **WHEN** listing `internal/model/` directory
- **THEN** files SHALL include: `extracted.go`, `rag_doc.go`, `llm.go`, `event.go`, `memory.go`, `errors.go`

### Requirement: All types remain in package model

All types SHALL remain in `package model` — no new packages introduced, no import path changes.

#### Scenario: Existing imports still compile
- **WHEN** building the project
- **THEN** all `model.SomeType` references SHALL resolve correctly

### Requirement: No cyclic dependencies

The split SHALL NOT introduce circular imports between packages.

#### Scenario: go vet passes
- **WHEN** running `go vet ./internal/model/`
- **THEN** no circular dependency errors SHALL appear

### Requirement: extracted.go

`internal/model/extracted.go` SHALL contain:
- `ExtractedContent`
- `Section`
- `ExtractedImage`

### Requirement: rag_doc.go

`internal/model/rag_doc.go` SHALL contain:
- `RAGDocument`
- `DocumentMetadata`
- `Claim`
- `KeyTerm`

### Requirement: llm.go

`internal/model/llm.go` SHALL contain:
- `ChatRequest`
- `ChatResponse`
- `Message`
- `StreamDelta`
- `TypedBlock`

### Requirement: event.go

`internal/model/event.go` SHALL contain:
- `Event`

### Requirement: memory.go

`internal/model/memory.go` SHALL contain:
- `MemoryEntry`

### Requirement: errors.go

`internal/model/errors.go` SHALL contain:
- `ErrExtractionFailed`
- `ErrLLMUnavailable`
- `ErrConfigInvalid`
- `ErrFileNotFound`
- `ErrInvalidInput`
