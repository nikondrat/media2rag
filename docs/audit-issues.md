# Audit Issues — media2rag v2

> Generated from architecture audit. Each issue is tracked here until resolved.
> Last updated: 2025-05-25

---

## Critical

### C1: rdrr External Dependency
**Files:** `extraction.md`, `deployment.md`
**Risk:** High
**Status:** ✅ Accepted — rdrr stays

`npx rdrr` — Node.js subprocess. Это затратно дублировать и бессмысленно — rdrr уже обрабатывает все URL типы.

**Решение:**
- v1: rdrr как зависимость (принято)
- v2+: Go-native fallback если rdrr недоступен
- Валидация формата вывода rdrr

---

### C2: No Authentication on Serve Mode
**Files:** `http-api.md`, `architecture.md`
**Risk:** High
**Status:** ⏳ Open — нужно для serve mode

HTTP API (`media2rag serve`) без auth. Для личного использования на localhost — не критично. Для serve mode с `--host 0.0.0.0` — нужно.

**Решение:**
- API key middleware для `/api/*`
- Config: `server.api_key` (env: `MEDIA2RAG_API_KEY`)
- Для личного use case на localhost — можно отложить

---

### C3: LLM Cost Explosion
**Files:** `ctg-pipeline.md`, `rag-engine.md`
**Risk:** Medium
**Status:** ⏳ Open

Pipeline делает десятки LLM-вызовов на файл. Нет лимитов, нет трекинга стоимости.

**Решение:**
- Token budget per operation
- `Usage` tracking в LLM responses
- SQLite таблица `usage_stats` (см. `sqlite-schema.md`)
- Предупреждение при превышении бюджета

---

## High

### H1: Goroutine Leaks / No Graceful Shutdown
**Files:** `architecture.md`, `http-api.md`
**Risk:** Medium
**Status:** ⏳ Open

Worker pool без graceful shutdown. Нет context cancellation. `StreamChat` возвращает channel — неясно кто закрывает.

**Решение:**
- Context cancellation propagation
- Worker pool с per-chunk timeout
- Graceful shutdown в serve mode
- Channel ownership documentation

---

### H2: SQLite Schema Not Designed
**Files:** `chat.md`, `architecture.md`
**Risk:** Medium
**Status:** ✅ Resolved — `docs/sqlite-schema.md`

Схема создана: sessions, messages, memories, documents, document_versions, skills, usage_stats. Миграции через `migrations/` директорию.

---

### H3: No Observability
**Files:** `architecture.md`
**Risk:** Medium
**Status:** ⏳ Open

Нет structured logging, tracing, metrics. Когда pipeline из 20 LLM-вызовов падает на 17-м — как дебажить?

**Решение:**
- Structured logging (JSON, levels, correlation IDs)
- Request tracing (trace ID per operation)
- Metrics: LLM call count, latency, error rate
- Debug mode: dump all LLM requests/responses

---

### H4: Quality Check Undefined
**Files:** `cli.md`, `ctg-pipeline.md`
**Risk:** Medium
**Status:** ⏳ Open

`--quality-check` есть, но нет критериев.

**Решение:**
- LLM-eval промпт (completeness, coherence, accuracy)
- Auto-revision on fail (max N retries)
- Quality score в metadata документа

---

## Medium

### M1: Coaching Engine Empty
**Files:** `architecture.md`, `roadmap.md`
**Risk:** Low
**Status:** ⏳ Deferred — не приоритет для Core Engine

`internal/coach/` упомянут, но нет дизайна. Отложено до после Core Engine.

---

### M2: No Test Strategy
**Files:** All
**Risk:** Medium
**Status:** ⏳ Open

Нет unit/integration/e2e тестов.

**Решение:**
- Mock LLM client для детерминированных тестов
- Golden file tests для pipeline output
- Test fixtures (sample markdown, expected output)

---

### M3: LanceDB vs Qdrant Inconsistency
**Files:** `roadmap.md`, `architecture.md`
**Risk:** Low
**Status:** ✅ Resolved

Qdrant — единственное решение. Все упоминания LanceDB удалены из roadmap.md.

---

### M4: Error Handling Gaps
**Files:** `architecture.md`
**Risk:** Medium
**Status:** ⏳ Open

Нет retry logic, circuit breaker, exponential backoff.

**Решение:**
- Retry с backoff для transient LLM errors
- Circuit breaker pattern
- Error categorization: retryable vs non-retryable

---

### M5: Embedding Model Switching
**Files:** `vector-store.md`
**Risk:** Medium
**Status:** ⏳ Open

Смена embedding модели требует реиндексации.

**Решение:**
- Collection naming by model: `parents_qwen3-1024`
- Reindex command
- Warning при model mismatch

---

## Low

### L1: CORS Wildcard
**Files:** `http-api.md`
**Risk:** Low
**Status:** ⏳ Open

`Access-Control-Allow-Origin: "*"` — должно быть настраиваемым.

---

### L2: No Health Check Depth
**Files:** `http-api.md`
**Risk:** Low
**Status:** ⏳ Open

`/health` не проверяет коллекции, модели, workspace.

---

### L3: Workspace Cleanup Edge Cases
**Files:** `workspace.md`
**Risk:** Low
**Status:** ⏳ Open

Нет transactional guarantee между Qdrant delete и filesystem delete.

---

## Discussion Outcomes

### Анализ звонков / B2B
- **Статус:** Не приоритет. Bitrix24, Gong уже делают это.
- **Проблема:** Нет CRM интеграции, dashboard, UI для продаж.
- **Решение:** Сфокусироваться на Core Engine для личного использования.

### Скилл-система / Маркетплейс
- **Статус:** Не приоритет. Дизайн записан (`docs/skill-system.md`) для будущего.
- **Решение:** Сначала Core Engine, потом скиллы.

### Core Engine Priority
- **Фокус:** process + chat + memory для личного использования
- **Use cases:** книги, видео, заметки, документы
- **Success:** 50+ документов, точные ответы с источниками, память между сессиями

---

## Summary

| Priority | Count | Open | Resolved | Deferred |
|----------|-------|------|----------|----------|
| Critical | 3 | 2 | 0 | 1 (accepted) |
| High | 4 | 3 | 1 | 0 |
| Medium | 5 | 4 | 1 | 1 |
| Low | 3 | 3 | 0 | 0 |
| **Total** | **15** | **12** | **2** | **2** |
