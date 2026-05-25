## ADDED Requirements

### Requirement: Process endpoint
`POST /api/process` SHALL accept source, start processing, return task_id for streaming.

#### Scenario: Submit process
- **WHEN** POST /api/process with `{"source": "https://..."}` 
- **THEN** 202 Accepted with `{"task_id": "abc123", "status": "queued"}`

### Requirement: Query endpoint
`POST /api/query` SHALL execute RAG pipeline and return answer with sources.

#### Scenario: RAG query
- **WHEN** POST /api/query with `{"question": "..."}` 
- **THEN** 200 OK with `{"answer": "...", "sources": [...]}`

### Requirement: Session endpoints
`POST /api/sessions`, `GET /api/sessions`, `DELETE /api/sessions/:id` SHALL manage chat sessions.

#### Scenario: Create session
- **WHEN** POST /api/sessions with `{"title": "..."}` 
- **THEN** 201 Created with session_id

#### Scenario: List sessions
- **WHEN** GET /api/sessions
- **THEN** 200 OK with sessions array

### Requirement: Chat message endpoint
`POST /api/sessions/:id/messages` SHALL add message and start streaming response.

#### Scenario: Send message
- **WHEN** POST /api/sessions/:id/messages with `{"content": "..."}` 
- **THEN** 202 Accepted with message_id

### Requirement: Documents endpoints
`GET /api/documents`, `DELETE /api/documents/:id` SHALL manage workspace documents.

#### Scenario: List documents
- **WHEN** GET /api/documents
- **THEN** 200 OK with documents array

### Requirement: Memory endpoints
`POST /api/memory`, `GET /api/memory` SHALL manage memory facts.

#### Scenario: Add memory
- **WHEN** POST /api/memory with `{"content": "...", "category": "fact"}` 
- **THEN** 201 Created

#### Scenario: Search memory
- **WHEN** GET /api/memory?query=HNSW&top_k=5
- **THEN** 200 OK with matching memories
