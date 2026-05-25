## Context

После Фазы 6 система полностью функциональна. Фаза 7 добавляет observability и тесты для production-качества.

**Constraints:**
- Standard library `log/slog` для structured logging
- Mock LLM для тестов без Ollama
- Golden files для pipeline output verification
- Integration tests skip если external services недоступны

## Goals / Non-Goals

**Goals:**
- Structured logging (slog) с request IDs
- Request tracing через pipeline stages
- Metrics: LLM calls, latency, errors
- Mock LLM client для unit tests
- Golden file tests для CTG pipeline
- Unit tests для всех пакетов
- Integration tests (skip без Ollama/Qdrant)

**Non-Goals:**
- Prometheus/Grafana — overkill для v1
- Distributed tracing (Jaeger) — single binary
- E2E tests с GUI — отдельный проект

## Decisions

### 1. slog вместо zap/logrus
**Why:** Standard library, Go 1.21+, no dependencies. Достаточно для single binary.
**Alternatives considered:** `uber-go/zap` — быстрее, но зависимость.

### 2. Mock LLM: hardcoded responses
**Why:** Простой mock с предзаписанными ответами. Golden files проверяют формат output.
**Alternatives considered:** Test LLM server — сложнее, не нужно.

### 3. Golden files для pipeline
**Why:** Фиксирует ожидаемый output. При изменении промпта — явное обновление golden.
**Alternatives considered:** Assertion-based tests — хрупкие, ломаются при изменении LLM output.

### 4. Integration tests с skip
**Why:** `testutil.RequireOllama(t)` — skip если сервис недоступен. CI может запустить с сервисами.
**Alternatives considered:** Docker compose для тестов — сложнее для dev.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Golden files diverge | `UPDATE_GOLDEN=1` flag |
| Mock LLM не реалистичен | Integration tests catch real issues |
| Test slowdown | Parallel tests, cached mocks |
