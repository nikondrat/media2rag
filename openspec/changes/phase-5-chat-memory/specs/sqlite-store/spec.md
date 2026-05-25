## ADDED Requirements

### Requirement: SQLite database initialization
The system SHALL initialize SQLite database at `~/.media2rag/data.db` (or configured path).

#### Scenario: Database creation
- **WHEN** store is initialized
- **THEN** database file is created with tables: sessions, messages, memory_facts

#### Scenario: WAL mode
- **WHEN** database is opened
- **THEN** WAL journal mode is enabled for concurrency

### Requirement: Session CRUD
The system SHALL create, read, update, delete chat sessions.

#### Scenario: Create session
- **WHEN** `CreateSession(title)` is called
- **THEN** new session with UUID, title, timestamps is stored

#### Scenario: List sessions
- **WHEN** `ListSessions()` is called
- **THEN** sessions ordered by updated_at DESC are returned

### Requirement: Message storage
The system SHALL store messages with session_id, role, content, sources, timestamp.

#### Scenario: Store message
- **WHEN** `AddMessage(sessionID, role, content, sources)` is called
- **THEN** message is stored with UUID and timestamp

#### Scenario: Get session history
- **WHEN** `GetMessages(sessionID, limit)` is called
- **THEN** messages ordered by created_at are returned
