# Issues & Roadmap — media2rag

> Generated: 2026-06-09
> Status: Active

---

## Scope Changes

### ❌ Removed from scope (never building)
- `chat` command — не будет
- `memory` system — не будет
- Qdrant integration — не будет
- MCP server — не будет
- B2B SaaS — не будет
- CRM интеграции — не будет
- Dashboard/UI — не будет

### ✅ What we build
- `process` — обработка файлов/URL/директорий в RAG-ready формат
- CTG Pipeline (Compress → Split → Process → Assemble)
- Batch processing с resume
- File logging + progress bar
- Health check (LLM alive, model loaded)

---

## 🔴 P0 — Critical Bugs (must fix now)

### 1. HumanEmitter — race condition ✅ FIXED
**File:** `internal/events/emitter.go`

Добавлен `sync.Mutex` в `HumanEmitter`.

### 2. `processSingle` — нет таймаута на LLM ✅ FIXED
**File:** `internal/pipeline/processor.go:146`

Добавлен `p.timeoutCtx(ctx)` в `processSingle`.

### 3. OpenRouterClient — HTTP без таймаута ✅ FIXED
**File:** `internal/llm/openrouter.go:27`

Добавлен `Timeout: 300 * time.Second`.

### 4. Checkpoint — ошибки молча игнорируются ✅ FIXED
**Files:** `internal/pipeline/splitter.go`, `processor.go`

`saveChunks`, `saveResults` теперь возвращают ошибки.

### 5. Panic recovery в processFile ✅ FIXED
**File:** `cmd/media2rag/process.go`

Добавлен `defer recover()` + запись ошибки в лог. Убраны дублирующие `emitter.Done()`.

---

## 🟡 P1 — Usability

### 6. Output directory — рядом с исходниками ✅ FIXED
По умолчанию: `<source-dir>/media2rag-output/<filename>/`
Структура: `output/final.md`, `chunks/`, `intermediate/`, `process.log`

### 7. Batch resume — skip processed ✅ FIXED
Если `output/final.md` существует — skip. `--force` для перезаписи.
Прогресс: `[3/200] processing: filename.md`

### 8. Health check ✅ FIXED
`media2rag health` — проверка LMStudio/Ollama + модель загружена.

### 9. Progress bar + ETA ⏳ TODO

---

## 🟢 P2 — Improvements

### 13. Retry с exponential backoff
LLM вызовы могут падать (timeout, rate limit). Нужен retry.

### 14. Парсинг LLM-ответов — fragile
`parsePromptResult` делает `strings.HasPrefix` на строках. Если модель отвечает иначе — поля пустые.

**Fix:** Более robust parsing, validation required fields.

### 15. `--dry-run` mode
Показать что будет обработано, без запуска LLM.

### 16. Конкурентность на уровне файлов
`processDirectory` — последовательная обработка. Для 200 файлов можно параллельно (с ограничением).

### 17. Статистика после batch
- Сколько файлов обработано
- Сколько ошибок
- Общее время
- Среднее время на файл
- Токены использованы

---

## Architecture Decisions

### Workspace vs Output Directory

| Aspect | Workspace (current) | Output Directory (proposed) |
|--------|-------------------|---------------------------|
| Location | `~/.media2rag/workspace/<hash>/` | `<source-dir>/media2rag-output/` |
| Readable | Нет (хеш) | Да (имя файла) |
| Resume | Да | Да (через final.md) |
| Checkpoint | Да | Да (через checkpoint dir) |
| User visible | Нет | Да |

**Decision:** Использовать оба. Workspace — для checkpoint/resume (внутренний). Output directory — для результатов (видимый пользователю).

### Pipeline Checkpoint

Pipeline уже имеет checkpoint систему:
- `compressed.md` — результат compress
- `chunks/chunk-NNN.md` — результат split
- `results.json` — результат process

**Problem:** checkpoint dir внутри workspace (неудобно).

**Fix:** checkpoint dir в output-dir каждого файла.

---

## Implementation Plan

### Phase 1: Fix Critical Bugs ✅ DONE
1. Thread-safe HumanEmitter ✅
2. Timeout в processSingle ✅
3. HTTP timeout в OpenRouterClient ✅
4. Checkpoint error handling ✅
5. Panic recovery ✅

### Phase 2: Output + Resume ✅ DONE
6. Output directory restructuring ✅
7. Batch resume (skip processed) ✅
8. Health check ✅
10. File logging (default) ✅

### Phase 3: UX ⏳ TODO
9. Progress bar + ETA
11. Recursive directory support
12. Model() on clients

### Phase 4: Polish ⏳ TODO
13. Retry logic
14. Robust LLM parsing
15. Batch statistics
16. File-level concurrency
17. Dry-run mode
