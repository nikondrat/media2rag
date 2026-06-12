## ADDED Requirements

### Requirement: HTTP server starts on configured host:port

The system SHALL start an HTTP server when running `media2rag serve`.

#### Scenario: Server starts successfully
- **WHEN** running `media2rag serve`
- **THEN** server SHALL listen on `--host` and `--port` (default localhost:8542)
- **AND** SHALL respond to `GET /api/health` with `{"status":"ok"}`

### Requirement: internal/api package contains router and handlers

The system SHALL have `internal/api/` package with HTTP logic separated from CLI.

#### Scenario: Router is initialized
- **WHEN** `api.Start()` is called
- **THEN** routes SHALL be registered for: `/api/health`, `/api/process`, `/api/query`, `/api/debug/*`

### Requirement: Serve command is thin wrapper

`cmd/media2rag/serve.go` SHALL be a thin CLI wrapper that calls `internal/api`.

#### Scenario: Serve delegates to api package
- **WHEN** user runs `media2rag serve`
- **THEN** `cmd/media2rag/serve.go` SHALL parse flags and call `api.Start()`
- **AND** SHALL not contain HTTP logic directly

### Requirement: Graceful shutdown

The server SHALL handle SIGINT/SIGTERM with graceful shutdown.

#### Scenario: SIGINT stops server
- **WHEN** server receives SIGINT
- **THEN** it SHALL finish in-flight requests within a timeout
- **AND** SHALL exit cleanly

### Requirement: API key middleware

Routes under `/api/` SHALL optionally require an API key if configured.

#### Scenario: API key validates
- **WHEN** `server.api_key` is set in config
- **THEN** requests without `Authorization: Bearer <key>` header SHALL get 401
- **AND** requests with valid key SHALL pass

#### Scenario: API key not configured
- **WHEN** `server.api_key` is not set
- **THEN** all requests SHALL pass without authentication

### Requirement: status command checks Qdrant

`media2rag status` SHALL check Qdrant connectivity in addition to Ollama and rdrr.

#### Scenario: Qdrant check in status output
- **WHEN** running `media2rag status`
- **THEN** output SHALL include Qdrant status line
- **AND** it SHALL show "ok" if connected, "error" if not reachable
