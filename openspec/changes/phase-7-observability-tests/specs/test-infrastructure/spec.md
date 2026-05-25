## ADDED Requirements

### Requirement: Mock LLM client
The test infrastructure SHALL provide a mock LLM client with pre-configured responses.

#### Scenario: Mock chat response
- **WHEN** mock `Chat()` is called
- **THEN** pre-configured response is returned

#### Scenario: Mock embed response
- **WHEN** mock `Embed()` is called
- **THEN** fixed-size float32 vector is returned

### Requirement: Golden file testing
The system SHALL support golden file tests for pipeline output verification.

#### Scenario: Golden file match
- **WHEN** pipeline output matches golden file
- **THEN** test passes

#### Scenario: Golden file update
- **WHEN** `UPDATE_GOLDEN=1` is set
- **THEN** golden files are regenerated

### Requirement: Test fixtures
Test fixtures SHALL provide sample documents for testing.

#### Scenario: Sample markdown
- **WHEN** test needs sample content
- **THEN** fixture markdown file is available
