## ADDED Requirements

### Requirement: Format detection
The system SHALL detect input format (question, command, statement, fragment) using heuristics without LLM.

#### Scenario: Question detection
- **WHEN** input ends with `?` or starts with "как", "что", "почему"
- **THEN** format is `question`

#### Scenario: Command detection
- **WHEN** input starts with "напиши", "объясни", "расскажи"
- **THEN** format is `command`

### Requirement: Semantic rewrite
The system SHALL rewrite user query into search-optimized format via LLM (1 call).

#### Scenario: Rewrite question
- **WHEN** format is question
- **THEN** query is rephrased as clear search terms (10-20 words)

### Requirement: Multi-query expansion
The system SHALL generate 3 alternative queries via LLM (1 call) for broader search coverage.

#### Scenario: Generate alternatives
- **WHEN** `Expand(query)` is called
- **THEN** 3 queries covering different aspects are returned
