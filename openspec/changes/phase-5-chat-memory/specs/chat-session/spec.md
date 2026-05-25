## ADDED Requirements

### Requirement: Session management
The Chat session SHALL manage message history, context window, and RAG enrichment.

#### Scenario: New session
- **WHEN** `NewSession()` is called
- **THEN** session is created with empty history

#### Scenario: Load session
- **WHEN** `LoadSession(id)` is called
- **THEN** session with message history is loaded from SQLite

### Requirement: Context window management
The system SHALL include last 5 messages as full text, older messages as summary.

#### Scenario: Recent history
- **WHEN** session has 10 messages
- **THEN** last 5 are included as full text

#### Scenario: Summarize old messages
- **WHEN** messages 1-5 exist
- **THEN** they are summarized into 1-2 sentences via LLM
