## ADDED Requirements

### Requirement: CORS headers
The server SHALL set CORS headers on all responses for web client compatibility.

#### Scenario: CORS on response
- **WHEN** any request is made
- **THEN** `Access-Control-Allow-Origin: *` header is set

#### Scenario: OPTIONS preflight
- **WHEN** OPTIONS request is made
- **THEN** 204 No Content with CORS headers is returned
