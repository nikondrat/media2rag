## ADDED Requirements

### Requirement: HTTP server startup
The system SHALL start HTTP server on configured host:port with graceful shutdown.

#### Scenario: Server starts
- **WHEN** `media2rag serve` is executed
- **THEN** server listens on localhost:8542

#### Scenario: Graceful shutdown
- **WHEN** SIGTERM is received
- **THEN** server finishes in-flight requests and exits

### Requirement: Route registration
The server SHALL register all REST endpoints and SSE stream handlers.

#### Scenario: Routes registered
- **WHEN** server starts
- **THEN** all /api/* and /health routes are accessible
