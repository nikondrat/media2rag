## ADDED Requirements

### Requirement: Request tracing
The system SHALL trace request flow through pipeline stages with timing.

#### Scenario: Trace pipeline
- **WHEN** process request runs
- **THEN** each stage (compress, split, process, assemble) is traced with duration

#### Scenario: Trace context
- **WHEN** trace is active
- **THEN** request_id propagates through all stages
