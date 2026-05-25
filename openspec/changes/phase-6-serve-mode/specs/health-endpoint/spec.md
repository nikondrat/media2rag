## ADDED Requirements

### Requirement: Health check endpoint
`GET /health` SHALL return status of Qdrant, Ollama, and server version.

#### Scenario: All services healthy
- **WHEN** GET /health with all services running
- **THEN** 200 OK with `{"status":"ok","qdrant":"connected","ollama":"connected"}`

#### Scenario: Service down
- **WHEN** Qdrant is not running
- **THEN** response shows `"qdrant":"disconnected"`
