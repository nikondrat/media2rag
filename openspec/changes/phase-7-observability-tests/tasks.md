## 1. Structured Logging (`internal/observe/log.go`)

- [ ] 1.1 Initialize slog with JSON handler
- [ ] 1.2 Implement log levels: debug, info, warn, error
- [ ] 1.3 Implement request ID generation and propagation
- [ ] 1.4 Wire logging into all packages
- [ ] 1.5 Implement `--verbose` flag → debug level

## 2. Request Tracing (`internal/observe/trace.go`)

- [ ] 2.1 Implement trace context with request_id
- [ ] 2.2 Implement stage timing (compress, split, process, assemble)
- [ ] 2.3 Implement trace output (log or separate trace store)

## 3. Metrics (`internal/observe/metrics.go`)

- [ ] 3.1 Implement LLM call counter
- [ ] 3.2 Implement latency tracking (histogram)
- [ ] 3.3 Implement error counter
- [ ] 3.4 Implement metrics export (log or endpoint)

## 4. Test Infrastructure

- [ ] 4.1 Implement MockLLMClient with pre-configured responses
- [ ] 4.2 Implement mock Embed returning fixed-size vector
- [ ] 4.3 Implement golden file testing utility
- [ ] 4.4 Create test fixtures: sample markdown, expected pipeline output
- [ ] 4.5 Implement `testutil.RequireOllama(t)` for integration test skip

## 5. Unit Tests

- [ ] 5.1 Tests for `internal/config/` — loading, merging, validation
- [ ] 5.2 Tests for `internal/extract/` — URL detection, local reading, rdrr parsing
- [ ] 5.3 Tests for `internal/pipeline/` — split, assemble, mock processing
- [ ] 5.4 Tests for `internal/rag/` — rewrite, RRF, dedup, context build
- [ ] 5.5 Tests for `internal/workspace/` — hash, CRUD, versioning
- [ ] 5.6 Tests for `internal/chat/` — session, context build
- [ ] 5.7 Tests for `internal/memory/` — store, recall

## 6. Integration Tests

- [ ] 6.1 Process end-to-end test (skip without Ollama)
- [ ] 6.2 RAG query end-to-end test (skip without Ollama + Qdrant)
- [ ] 6.3 Chat session end-to-end test (skip without Ollama + Qdrant)

## 7. CI Readiness

- [ ] 7.1 `go test ./...` passes with mock LLM
- [ ] 7.2 `go vet ./...` passes
- [ ] 7.3 `go fmt ./...` passes
