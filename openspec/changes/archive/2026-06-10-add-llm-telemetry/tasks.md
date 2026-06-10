## 1. Model Layer

- [x] 1.1 Add `Usage` struct to `internal/model/llm.go` with PromptTokens, CompletionTokens, TotalTokens
- [x] 1.2 Add `Usage *Usage` field to `ChatResponse` struct
- [x] 1.3 Create `internal/model/telemetry.go` with `LLMTelemetry` struct and `TelemetryRecorder` interface
- [x] 1.4 Add `TeeRecorder` implementation in `internal/model/telemetry.go`

## 2. Provider Parsing — Token Usage

- [x] 2.1 Add `PromptEvalCount` and `EvalCount` fields to `ollamaChatResponse` struct in `internal/llm/ollama.go`
- [x] 2.2 Parse token usage from Ollama response into `ChatResponse.Usage`
- [x] 2.3 Add `Usage` struct to `openRouterResponse` in `internal/llm/openrouter.go`
- [x] 2.4 Parse token usage from OpenRouter response into `ChatResponse.Usage`

## 3. Context Keys

- [x] 3.1 Create `internal/llm/telemetry.go` with context key types and helper functions: `WithStage`, `WithChunkIndex`, `WithSource`, `WithRetryAttempt`
- [x] 3.2 Add extraction functions: `stageFromCtx`, `chunkFromCtx`, `sourceFromCtx`, `retryFromCtx`
- [x] 3.3 Add `promptChars()` helper to calculate total chars from Messages

## 4. InstrumentedClient

- [x] 4.1 Create `internal/llm/instrumented.go` with `InstrumentedClient` struct wrapping `LLMClient`
- [x] 4.2 Implement `InstrumentedClient.Chat()` with latency measurement, usage extraction, cost calculation, and telemetry recording
- [x] 4.3 Implement `InstrumentedClient.StreamChat()` with content accumulation and final telemetry recording
- [x] 4.4 Add constructor `NewInstrumentedClient(inner, pricing, recorder)`

## 5. Recorders

- [x] 5.1 Create `internal/pipeline/telemetry.go` with `JSONLRecorder` (thread-safe, append-only, writes to `<output>/telemetry.jsonl`)
- [x] 5.2 Add `StatusRecorder` that aggregates telemetry into `PipelineStatus` (chunk cost, stage breakdown, totals)

## 6. Status Model Updates

- [x] 6.1 Add `Cost`, `TokensIn`, `TokensOut`, `Model`, `LatencyMs` fields to `ChunkStatus` in `internal/pipeline/status.go`
- [x] 6.2 Add `TotalCost`, `TotalTokensIn`, `TotalTokensOut`, `StageBreakdown` to `PipelineStatus`
- [x] 6.3 Add `StageCost` struct with Calls, Cost, TokensIn, TokensOut
- [x] 6.4 Fix `ChunkDone()` to preserve existing fields instead of overwriting
- [x] 6.5 Fix `ChunkFailed()` to preserve existing fields instead of overwriting
- [x] 6.6 Fix `SetChunks()` to preserve existing cost fields on resume

## 7. Pipeline Integration

- [x] 7.1 Add `WithStage(ctx, "pre_clean")` and `WithSource(ctx, p.source)` before each LLM call in `preClean()`
- [x] 7.2 Add `WithStage(ctx, "process")`, `WithChunkIndex(ctx, index)`, and `WithRetryAttempt(ctx, attempt)` in `processSingle()`
- [x] 7.3 Add `WithStage(ctx, "context_enrich")` and `WithChunkIndex(ctx, index)` in `enrichSingle()`
- [x] 7.4 Add `WithStage(ctx, "holistic")` in `holisticAnalysis()`
- [x] 7.5 Add `WithStage(ctx, "causal")` in `causalExtraction()`

## 8. CLI Integration

- [x] 8.1 Create `JSONLRecorder` and `StatusRecorder` in `cmd/media2rag/process.go` before pipeline starts
- [x] 8.2 Wrap existing LLM client with `NewInstrumentedClient(pricingStore, recorder)`
- [x] 8.3 Wire pricing store loading into pipeline initialization
- [x] 8.4 Ensure recorders are closed on pipeline completion or failure

## 9. Verify

- [x] 9.1 Build project: `go build ./cmd/media2rag`
- [ ] 9.2 Process a test file and verify `telemetry.jsonl` is created with valid JSONL entries (requires LLM backend)
- [ ] 9.3 Verify `status.yaml` contains cost/tokens/latency per chunk, total cost, and stage breakdown (requires LLM backend)
- [ ] 9.4 Verify `ChunkDone` does not wipe cost fields on resume (requires LLM backend)
- [ ] 9.5 Verify concurrent worker pool calls produce non-interleaved telemetry.jsonl (requires LLM backend)
