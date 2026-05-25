## ADDED Requirements

### Requirement: SSE event format
SSE streams SHALL emit events with `event: <type>` and `data: <JSON>` format.

#### Scenario: Token event
- **WHEN** LLM generates token
- **THEN** `event: token\ndata: {"token":"..."}` is sent

#### Scenario: Sources event
- **WHEN** response completes
- **THEN** `event: sources\ndata: [{"ref":1,"title":"..."}]` is sent

#### Scenario: Done event
- **WHEN** stream ends
- **THEN** `event: done\ndata: {}` is sent

### Requirement: Stream subscription
Clients SHALL subscribe to streams via `GET /api/stream/{id}`.

#### Scenario: Subscribe to process stream
- **WHEN** GET /api/stream/process-abc123
- **THEN** SSE connection opens and events flow

#### Scenario: Disconnect handling
- **WHEN** client disconnects
- **THEN** goroutine is cleaned up
