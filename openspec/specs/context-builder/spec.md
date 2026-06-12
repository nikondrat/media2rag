## ADDED Requirements

### Requirement: Build LLM prompt with sources
The ContextBuilder SHALL assemble system message with sources, context, and query.

#### Scenario: Format context
- **WHEN** chunks are provided
- **THEN** each is formatted as `Source [N]:\n> content`

#### Scenario: Source block
- **WHEN** sources are assembled
- **THEN** source block lists `[N]: title (type, source)`

#### Scenario: Citation instruction
- **WHEN** system prompt is built
- **THEN** it instructs LLM to cite sources as [1], [2]
