# llm-parse-integration Specification

## Purpose
TBD - created by archiving change llm-output-format. Update Purpose after archive.
## Requirements
### Requirement: ChatAndParse method
The LLMClient SHALL provide `ChatAndParse(ctx, prompt) ([]TypedBlock, error)` that calls Chat and parses response.

#### Scenario: Chat and parse
- **WHEN** `ChatAndParse` is called with a prompt
- **THEN** LLM is called, response is parsed, `[]TypedBlock` is returned

#### Scenario: Parse error fallback
- **WHEN** LLM returns plain text without format markers
- **THEN** single "text" block is returned, no error

### Requirement: StreamAndParse method
The LLMClient SHALL provide `StreamAndParse(ctx, prompt) (<-chan StreamDelta, chan []TypedBlock, error)` that streams tokens and returns parsed result on completion.

#### Scenario: Stream and parse
- **WHEN** `StreamAndParse` is called
- **THEN** tokens stream via delta channel, parsed blocks arrive on result channel when done

#### Scenario: Stream cancellation
- **WHEN** context is cancelled during streaming
- **THEN** channels are closed, goroutine exits

