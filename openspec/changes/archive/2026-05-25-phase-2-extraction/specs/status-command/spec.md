## ADDED Requirements

### Requirement: Status command health checks
The `status` command SHALL check availability of Ollama, rdrr, workspace, and display summary.

#### Scenario: All services available
- **WHEN** `media2rag status` is executed with all services running
- **THEN** it shows: Ollama connected, rdrr available, workspace path, document count

#### Scenario: Ollama unavailable
- **WHEN** Ollama is not running
- **THEN** it shows: "Ollama: not connected (localhost:11434)"

#### Scenario: rdrr not installed
- **WHEN** `npx rdrr` is not available
- **THEN** it shows: "rdrr: not found"

### Requirement: Status output format
The status command SHALL display results in human-readable format with connection status.

#### Scenario: Normal output
- **WHEN** `media2rag status` is executed
- **THEN** output shows each service with ✓ or ✗ indicator

#### Scenario: JSON output
- **WHEN** `media2rag status --json` is executed
- **THEN** output is JSON with service statuses

### Requirement: Status checks
The status command SHALL verify: Ollama HTTP connectivity, rdrr CLI availability, workspace directory existence, document count.

#### Scenario: Ollama connectivity check
- **WHEN** status checks Ollama
- **THEN** it sends GET to `http://localhost:11434/api/tags` and checks response

#### Scenario: rdrr availability check
- **WHEN** status checks rdrr
- **THEN** it runs `npx rdrr --version` or similar probe
