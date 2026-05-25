## ADDED Requirements

### Requirement: Qdrant client connection
The system SHALL connect to Qdrant via gRPC at configurable URL (default: localhost:6334).

#### Scenario: Successful connection
- **WHEN** Qdrant is running
- **THEN** client connects and can list collections

#### Scenario: Connection failure
- **WHEN** Qdrant is not running
- **THEN** client returns `ErrStoreUnavailable`

### Requirement: Collection management
The system SHALL create and manage collections: `documents` (for parent/child chunks) and `memories`.

#### Scenario: Create documents collection
- **WHEN** `InitCollections()` is called
- **THEN** `documents` collection exists with vector config

#### Scenario: Collection already exists
- **WHEN** collection already exists
- **THEN** no error is returned

### Requirement: Upsert points
The system SHALL upsert points with vector, payload (content, document_id, parent_id, content_hash).

#### Scenario: Insert child point
- **WHEN** child chunk is indexed
- **THEN** point has embedding, content, document_id, parent_id, content_hash

#### Scenario: Insert parent point
- **WHEN** parent chunk is indexed
- **THEN** point has embedding, full content, document_id, no parent_id

### Requirement: Search points
The system SHALL search points with vector similarity, returning topK results with payloads.

#### Scenario: Semantic search
- **WHEN** `SearchPoints(query, topK)` is called
- **THEN** topK most similar points are returned with scores

### Requirement: Delete points
The system SHALL delete points by document_id filter.

#### Scenario: Delete document points
- **WHEN** `DeletePoints(document_id)` is called
- **THEN** all points with that document_id are removed
