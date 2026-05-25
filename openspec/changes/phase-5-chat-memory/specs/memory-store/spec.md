## ADDED Requirements

### Requirement: Fact storage
The system SHALL store memory facts in Qdrant `memories` collection with content, user_id, created_at.

#### Scenario: Store fact
- **WHEN** `Store(userID, content, category)` is called
- **THEN** fact is embedded and stored in Qdrant

### Requirement: Fact recall
The system SHALL search facts by semantic similarity, returning topK relevant facts.

#### Scenario: Recall relevant facts
- **WHEN** `Recall(userID, query, topK)` is called
- **THEN** topK most relevant facts are returned

### Requirement: Fact CRUD
The system SHALL support create, read, delete, list operations for memory facts.

#### Scenario: Delete fact
- **WHEN** `Delete(entryID)` is called
- **THEN** fact is removed from Qdrant

#### Scenario: List facts
- **WHEN** `List(userID)` is called
- **THEN** all facts for user are returned
