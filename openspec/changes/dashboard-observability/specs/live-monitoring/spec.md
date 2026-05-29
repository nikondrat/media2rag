## ADDED Requirements

### Requirement: SSE event stream
The system SHALL provide a Server-Sent Events endpoint at `GET /api/debug/live` for real-time pipeline updates.

#### Scenario: SSE connection
- **WHEN** a client connects to `/api/debug/live`
- **THEN** the server sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and `Connection: keep-alive` headers

#### Scenario: Connected event
- **WHEN** a client successfully connects
- **THEN** the server sends `event: connected\ndata: {"status":"ok"}\n\n`

### Requirement: SSE event types
The system SHALL emit the following event types: pipeline_start, pipeline_complete, stage_complete, llm_call, judge_complete, feedback, and heartbeat.

#### Scenario: Pipeline events
- **WHEN** a pipeline starts
- **THEN** an SSE event `pipeline_start` is broadcast with run_id, source, and timestamp
- **WHEN** a pipeline completes
- **THEN** an SSE event `pipeline_complete` is broadcast with run_id, score, and latency_ms

#### Scenario: Stage event
- **WHEN** a pipeline stage completes
- **THEN** an SSE event `stage_complete` is broadcast with run_id, stage name, score, and latency_ms

#### Scenario: LLM call event
- **WHEN** an LLM call completes
- **THEN** an SSE event `llm_call` is broadcast with call_id, model, operation, tokens, and latency

#### Scenario: Judge event
- **WHEN** a judge evaluation completes
- **THEN** an SSE event `judge_complete` is broadcast with run_id, score, and judge_type

#### Scenario: Feedback event
- **WHEN** feedback is submitted
- **THEN** an SSE event `feedback` is broadcast with run_id and rating

#### Scenario: Heartbeat
- **WHEN** no events have been sent for 30 seconds
- **THEN** a heartbeat event with current timestamp is sent to keep the connection alive

### Requirement: SSE fan-out
The system SHALL support multiple concurrent SSE connections with event broadcasting to all connected clients.

#### Scenario: Multiple clients
- **WHEN** 3 clients are connected to `/api/debug/live`
- **THEN** all 3 receive the same events simultaneously

#### Scenario: Client disconnect
- **WHEN** a client disconnects
- **THEN** the server removes the client from the broadcast list with no errors to other clients

### Requirement: SSE reconnection
The dashboard frontend SHALL automatically reconnect to the SSE stream on connection loss.

#### Scenario: Auto-reconnect
- **WHEN** the SSE connection drops
- **THEN** the frontend reconnects after a 3-second delay
