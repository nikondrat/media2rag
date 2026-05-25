## ADDED Requirements

### Requirement: Structured logging
The system SHALL use `log/slog` for structured logging with levels (debug, info, warn, error).

#### Scenario: Log with fields
- **WHEN** `log.Info("processing", "source", url)` is called
- **THEN** JSON log line includes level, message, and fields

#### Scenario: Request ID
- **WHEN** request is processed
- **THEN** all log lines include `request_id` field

### Requirement: Log level configuration
Log level SHALL be configurable via config (default: info).

#### Scenario: Debug mode
- **WHEN** log level is set to debug
- **THEN** debug messages are output

#### Scenario: Verbose flag
- **WHEN** `--verbose` is passed
- **THEN** log level is set to debug
