## ADDED Requirements

### Requirement: API key authentication
The server SHALL validate `Authorization: Bearer <key>` header when auth is enabled.

#### Scenario: Valid API key
- **WHEN** request has valid Bearer token
- **THEN** request proceeds to handler

#### Scenario: Missing API key
- **WHEN** request has no Authorization header
- **THEN** 401 Unauthorized is returned

#### Scenario: Invalid API key
- **WHEN** request has invalid Bearer token
- **THEN** 401 Unauthorized is returned

### Requirement: Optional auth
Auth middleware SHALL be disabled when no API key is configured.

#### Scenario: No auth configured
- **WHEN** no API key in config
- **THEN** all requests proceed without auth check
