## ADDED Requirements

### Requirement: ExtractedContent model
The system SHALL define `ExtractedContent` struct with fields: Title, Author, Source, DocType, Content, Language, Sections, Images, Metadata, WordCount, CharCount.

#### Scenario: ExtractedContent creation
- **WHEN** content is extracted from a file
- **THEN** an `ExtractedContent` struct is populated with all fields

### Requirement: RAGDocument model
The system SHALL define `RAGDocument` struct with `Markdown` and `DocumentMetadata` fields.

#### Scenario: RAGDocument assembly
- **WHEN** pipeline completes
- **THEN** a `RAGDocument` is created with markdown content and metadata

### Requirement: DocumentMetadata model
The `DocumentMetadata` struct SHALL contain: Title, Author, Source, DocType, Language, Domains, CoreThesis, MentalModels, Claims, Takeaways, KeyTerms, Summary, KeyInsights, WordCount, Topics.

#### Scenario: Metadata populated
- **WHEN** document metadata is assembled
- **THEN** all fields are set with appropriate values

### Requirement: LLM request/response models
The system SHALL define `ChatRequest`, `ChatResponse`, `StreamDelta`, `Message` structs for LLM communication.

#### Scenario: ChatRequest construction
- **WHEN** a chat request is built
- **THEN** it contains model, messages, stream flag, and optional images

### Requirement: Event model
The `Event` struct SHALL have fields: Type (string), Data (any), Progress (float64), Error (string).

#### Scenario: Event serialization
- **WHEN** an Event is marshaled to JSON
- **THEN** it produces valid JSON with correct field names

### Requirement: MemoryEntry model
The `MemoryEntry` struct SHALL have fields: ID, UserID, Content, Category, CreatedAt.

#### Scenario: MemoryEntry creation
- **WHEN** a memory entry is stored
- **THEN** it is created with unique ID and timestamp
