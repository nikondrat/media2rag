## Why

После Serve Mode (Фаза 6) система полностью функциональна. Но для production-качества нужны: structured logging для дебага, request tracing для анализа производительности, metrics для мониторинга, и comprehensive test suite для уверенности при рефакторинге.

## What Changes

- `internal/observe/` — structured logging (slog), request tracing, metrics collection
- Test infrastructure: mock LLM client, golden files, test fixtures
- Unit tests для всех пакетов: config, extract, pipeline, rag, chat, memory, workspace
- Integration tests: process end-to-end, RAG query, chat session
- Golden file tests для CTG pipeline output

## Capabilities

### New Capabilities
- `structured-logging`: slog-based logging with levels, fields, request IDs
- `request-tracing`: trace request flow through pipeline stages
- `metrics-collection`: LLM call counts, latency, error rates
- `test-infrastructure`: mock LLM, golden file testing, test fixtures
- `unit-tests`: comprehensive unit test coverage for all packages

### Modified Capabilities
- All packages: add logging, tracing integration points

## Impact

- Зависимости: `testing`, `httptest` (stdlib), golden file fixtures
- CI-ready: `go test ./...` проходит без внешних сервисов (mock LLM)
- Integration tests требуют Ollama + Qdrant (skip если недоступны)
