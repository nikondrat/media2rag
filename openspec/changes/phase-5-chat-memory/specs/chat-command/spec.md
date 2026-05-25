## ADDED Requirements

### Requirement: Interactive terminal chat
The `chat` command SHALL start interactive readline loop, send messages, stream responses, display sources.

#### Scenario: Start chat
- **WHEN** `media2rag chat` is executed
- **THEN** interactive prompt appears, waiting for user input

#### Scenario: Continue session
- **WHEN** `media2rag chat --session abc123` is executed
- **THEN** existing session is loaded with history

#### Scenario: Stream response
- **WHEN** user sends message
- **THEN** LLM response streams token by token to terminal

#### Scenario: Display sources
- **WHEN** response completes
- **THEN** sources are displayed with [N] references
