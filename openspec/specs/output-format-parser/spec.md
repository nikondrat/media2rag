# output-format-parser Specification

## Purpose
TBD - created by archiving change llm-output-format. Update Purpose after archive.
## Requirements
### Requirement: ParseOutput function
The system SHALL provide `ParseOutput(text string) ([]TypedBlock, error)` that parses LLM responses in `> type(params)\ncontent\n<` format.

#### Scenario: Parse single block
- **WHEN** `ParseOutput("> memory\nПользователя зовут Никита\n<")` is called
- **THEN** returns `[]TypedBlock{{Type: "memory", Params: {}, Content: "Пользователя зовут Никита"}}`

#### Scenario: Parse block with params
- **WHEN** `ParseOutput("> topic: chunk=3, lang=ru\nHNSW, векторный поиск\n<")` is called
- **THEN** returns `[]TypedBlock{{Type: "topic", Params: {"chunk":"3","lang":"ru"}, Content: "HNSW, векторный поиск"}}`

#### Scenario: Parse multiple blocks
- **WHEN** input contains `> topic\ntopic1\n<\n> summary\nsummary text\n<`
- **THEN** returns two TypedBlocks in order

#### Scenario: Plain text fallback
- **WHEN** input has no `> type\n<` markers
- **THEN** returns single block with Type="text" and full content

#### Scenario: Missing closing marker
- **WHEN** block starts with `> type\n` but has no `<`
- **THEN** block closes at next `>` marker or EOF

#### Scenario: Nested > in content
- **WHEN** content contains `>` character (blockquote)
- **THEN** it is not confused with block marker (marker requires `> type\n` pattern)

### Requirement: TypedBlock model
The `TypedBlock` struct SHALL have fields: Type (string), Params (map[string]string), Content (string).

#### Scenario: TypedBlock creation
- **WHEN** block is parsed
- **THEN** all fields are populated correctly

