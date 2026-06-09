## MODIFIED Requirements

### Requirement: Per-chunk LLM processing
The Processor SHALL send each chunk to LLM with parallel prompts for: title, topics, summary, claims, mental models, key terms, and takeaways.

#### Scenario: Multi-prompt extraction
- **WHEN** LLM processes a chunk
- **THEN** it receives separate prompts for each extraction type (claims, mental_models, key_terms, core_thesis, takeaways) in addition to the base prompt (title, topics, summary)

#### Scenario: Claims extraction
- **WHEN** claims prompt is sent
- **THEN** LLM returns claims in structured format

#### Scenario: Mental models extraction
- **WHEN** mental models prompt is sent
- **THEN** LLM returns mental model names

#### Scenario: Key terms extraction
- **WHEN** key terms prompt is sent
- **THEN** LLM returns key terms with definitions

#### Scenario: Core thesis extraction
- **WHEN** core thesis prompt is sent
- **THEN** LLM returns a single thesis statement

#### Scenario: Takeaways extraction
- **WHEN** takeaways prompt is sent
- **THEN** LLM returns actionable takeaways

#### Scenario: Configurable extraction levels
- **WHEN** PipelineConfig has ExtractClaims=false
- **THEN** claims prompt is skipped for all chunks
