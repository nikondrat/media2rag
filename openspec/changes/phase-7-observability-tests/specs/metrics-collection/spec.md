## ADDED Requirements

### Requirement: LLM call metrics
The system SHALL track LLM call count, latency, and error rate.

#### Scenario: Count LLM calls
- **WHEN** LLM client is called
- **THEN** call counter is incremented

#### Scenario: Track latency
- **WHEN** LLM call completes
- **THEN** duration is recorded

#### Scenario: Track errors
- **WHEN** LLM call fails
- **THEN** error counter is incremented
