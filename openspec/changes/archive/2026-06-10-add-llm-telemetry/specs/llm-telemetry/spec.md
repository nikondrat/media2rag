## ADDED Requirements

### Requirement: Usage in ChatResponse
The `ChatResponse` model SHALL include a `Usage` struct with `PromptTokens`, `CompletionTokens`, and `TotalTokens` fields.

#### Scenario: Ollama returns token counts
- **WHEN** `OllamaClient.Chat()` returns a successful response
- **THEN** `ChatResponse.Usage` SHALL contain `PromptTokens` from `prompt_eval_count` and `CompletionTokens` from `eval_count`

#### Scenario: OpenRouter returns token counts
- **WHEN** `OpenRouterClient.Chat()` returns a successful response
- **THEN** `ChatResponse.Usage` SHALL contain `PromptTokens`, `CompletionTokens`, and `TotalTokens` from the `usage` object

#### Scenario: Provider without usage data
- **WHEN** the provider response does not include token counts
- **THEN** `ChatResponse.Usage` SHALL be `nil` (InstrumentedClient falls back to zero-valued Usage)

### Requirement: Full telemetry per LLM call
The system SHALL record an `LLMTelemetry` entry for every LLM call, containing: source, stage, chunk index, retry attempt, model, prompt/completion tokens, prompt/completion chars, cost in USD, latency in ms, success status, error message, and timestamp.

#### Scenario: Successful Chat call records telemetry
- **WHEN** `InstrumentedClient.Chat()` returns a successful response
- **THEN** a complete `LLMTelemetry` entry is recorded with `Success: true` and all fields populated

#### Scenario: Failed Chat call records error telemetry
- **WHEN** `InstrumentedClient.Chat()` returns an error
- **THEN** an `LLMTelemetry` entry is recorded with `Success: false` and `Error` containing the error message

### Requirement: Context-based metadata propagation
Pipeline SHALL propagate stage, chunk index, source name, and retry attempt via `context.Context` using helper functions `WithStage`, `WithChunkIndex`, `WithSource`, `WithRetryAttempt`.

#### Scenario: Stage extracted from context
- **WHEN** `WithStage(ctx, "process")` is called before a Chat call
- **THEN** the recorded telemetry SHALL have `Stage: "process"`

#### Scenario: Chunk index extracted from context
- **WHEN** `WithChunkIndex(ctx, 3)` is called before a Chat call
- **THEN** the recorded telemetry SHALL have `ChunkIndex: 3`

#### Scenario: Source extracted from context
- **WHEN** `WithSource(ctx, "lecture.md")` is called before a Chat call
- **THEN** the recorded telemetry SHALL have `Source: "lecture.md"`

#### Scenario: Retry attempt extracted from context
- **WHEN** `WithRetryAttempt(ctx, 1)` is called before a Chat call
- **THEN** the recorded telemetry SHALL have `RetryAttempt: 1`

### Requirement: Cost calculation per call
The system SHALL calculate cost in USD for each LLM call using `CalculateCost(model, promptTokens, completionTokens)` from `pricing.go`.

#### Scenario: Known model calculates cost
- **WHEN** a Chat call uses a model with pricing data (e.g., `qwen/qwen3.5-9b`)
- **THEN** `Cost` SHALL equal `(promptTokens/1M * inputPrice) + (completionTokens/1M * outputPrice)`

#### Scenario: Unknown model returns zero cost
- **WHEN** a Chat call uses a model without pricing data
- **THEN** `Cost` SHALL be `0.0` (not an error)

### Requirement: JSONL output
The system SHALL write each LLM call telemetry as a JSON line to `<output-dir>/telemetry.jsonl`, append-only and thread-safe.

#### Scenario: Single call writes one line
- **WHEN** one LLM call completes
- **THEN** exactly one JSON line is appended to `telemetry.jsonl`

#### Scenario: Concurrent calls are thread-safe
- **WHEN** multiple LLM calls complete concurrently
- **THEN** each JSON line in `telemetry.jsonl` is complete and valid (not interleaved)

#### Scenario: Partial file on crash
- **WHEN** the process crashes mid-write
- **THEN** previously written lines in `telemetry.jsonl` remain valid JSON

### Requirement: Status aggregation in status.yaml
The system SHALL aggregate telemetry into `status.yaml` with per-chunk cost/tokens/latency, total cost/tokens, and per-stage breakdown.

#### Scenario: Per-chunk cost recorded
- **WHEN** a chunk processing LLM call completes
- **THEN** `status.yaml` chunks[index] SHALL include cost, tokens_in, tokens_out, model, latency_ms

#### Scenario: Total cost accumulates
- **WHEN** multiple LLM calls complete for one document
- **THEN** `status.yaml` total_cost SHALL equal sum of all call costs

#### Scenario: Stage breakdown tracked
- **WHEN** LLM calls complete for different stages
- **THEN** `status.yaml` stage_breakdown SHALL show calls count, cost, tokens per stage

### Requirement: ChunkDone preserves existing fields
The `ChunkDone()` method SHALL preserve existing cost/tokens/latency fields in `ChunkStatus` when marking a chunk as done.

#### Scenario: Cost preserved after ChunkDone
- **WHEN** `StatusRecorder` has set cost on a chunk and `ChunkDone(index)` is called
- **THEN** the chunk's cost SHALL remain unchanged

#### Scenario: Failed status preserves cost
- **WHEN** `ChunkFailed(index, errMsg)` is called
- **THEN** the chunk's cost/tokens/latency fields SHALL remain unchanged

### Requirement: StreamChat telemetry
The `InstrumentedClient.StreamChat()` SHALL accumulate streamed content and record telemetry when the stream completes.

#### Scenario: Ollama stream records usage
- **WHEN** `OllamaClient.StreamChat()` completes with `done: true` and `eval_count` in the final chunk
- **THEN** `LLMTelemetry` is recorded with token counts from the final chunk

#### Scenario: OpenAI stream records telemetry
- **WHEN** `OpenRouterClient.StreamChat()` completes
- **THEN** accumulated `CompletionChars` and latency are recorded (usage if available)
