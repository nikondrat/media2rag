# Issues & Roadmap — media2rag

> Updated: 2026-06-10
> Status: Active

---

## Current State (v1)

**Что работает стабильно:**
- `process` — обработка одного файла (`.md`, URL) через CTG Pipeline
- `process <directory>` — batch-обработка с параллельными файлами и resume
- CTG Pipeline: preClean → splitText → processChunks → holisticAnalysis → causalExtraction → contextualEnrich → assemble
- URL auto-detection через `npx rdrr` (веб, YouTube, GitHub, Telegram)
- Health check (LMStudio, Ollama, OpenRouter key)
- Retry logic: pipeline (2 retries) + OpenRouter (3 retries) + validation retry
- Progress bar (mpb) для batch режима (показывает файл/стадию/ETA)
- Output directory: `<dir>/media2rag-output/<name>/` с `output/final.md`, `chunks/`, `intermediate/`
- Workspace: `~/.media2rag/workspace/<hash>/` с версионированием
- Telemetry: JSONL запись всех LLM вызовов (токены, стоимость, latency)
- Документы: `documents list|show|delete`
- Параллельная обработка файлов (file_concurrency: OpenRouter=3, LMStudio=4, total_concurrency: OpenRouter=100, LMStudio=16)

**Что в uncommitted changes (активная разработка):**
- Splitter: параграф-ориентированное разбиение (вместо sliding window)
- Processor: retry + валидация + template detection
- Status: per-chunk cost/tokens/latency tracking, stage breakdown
- Pricing: динамическая загрузка цен моделей с API
- Progress: улучшенный ETA (per-file average вместо wall clock)
- GraphRAG docs

---

## ✅ Done & Stable

### P0 — Critical Bugs (all fixed)
1. **HumanEmitter race condition** — `sync.Mutex` добавлен
2. **Timeout в processSingle** — `p.timeoutCtx(ctx)` добавлен
3. **HTTP timeout OpenRouter** — `Timeout: 300s`
4. **Checkpoint errors silenced** — `saveChunks`, `saveResults` возвращают ошибки
5. **Panic recovery** — `defer recover()` в processFile

### P1 — Usability (all done)
6. **Output directory** — `<source>/media2rag-output/<name>/`
7. **Batch resume** — skip если `status.yaml: stage=done`, `--force` для перезапуска
8. **Health check** — `media2rag health` (LMStudio, Ollama, OpenRouter)
9. **Progress bar + ETA** — mpv bar с файлом/стадией/ETA для batch
10. **File logging** — `TeeEmitter` пишет `process.log` в output dir

### Core Features
- **CTG Pipeline** — Compress → Split → Process → Assemble с checkpoint/resume
- **Holistic analysis** — core thesis + domains из LLM
- **Causal extraction** — causal chains, preconditions, counterfactuals
- **Context enrichment** — per-chunk контекст в документе
- **URL processing** — `npx rdrr` для любых URL
- **File-level concurrency** — `--file-concurrency` / `config.pipeline.max_file_concurrency`
- **LLM concurrency** — `--total-concurrency` / `config.pipeline.max_total_concurrency`
- **Multiple backends** — LMStudio, Ollama, OpenRouter (с fallback chain)
- **Telemetry** — JSONL запись всех LLM вызовов в output dir
- **Pricing** — динамическая загрузка цен моделей (cached 15 min)
- **Documents management** — `documents list|show|delete`

---

## ⏳ In Progress / Uncommitted

- Splitter rewrite (paragraph-based instead of sliding window)
- Processor retry validation + template detection
- Pipeline status enriched (cost, tokens, stage breakdown)
- Dynamic pricing API

---

## 🟡 P2 — TODO (next)

### 13. Retry logic ✅ DONE
- Pipeline: `cleanSinglePart` retry (2 attempts, backoff 2s, 8s) ✅
- Pipeline: `processSingle` retry (2 attempts, validation + template detection) ✅
- OpenRouter: `doWithRetry` (3 attempts, backoff 2s, 8s, 18s) ✅
- Fallback chain: `tryChain` (sequential models on retryable errors) ✅
- **Что ещё нужно:** retry для holistic, causal, context enrichment этапов

### 14. Robust LLM parsing ✅ IMPROVED (не идеально)
- `safeFieldValue` — bounds checking против пустых строк ✅
- `validateChunkResult` — проверка type, template detection (`<\w+>`) ✅
- Retry с `retryPromptSuffix` при invalid ответе ✅
- **Что ещё нужно:** multi-line summary/key_points (сейчас только первая строка)

### 15. `--dry-run` mode ⏳ TODO
Показать что будет обработано, без запуска LLM.

### 16. File-level concurrency ✅ DONE
- Config: `pipeline.max_file_concurrency` ✅
- Config: `pipeline.max_total_concurrency` ✅
- CLI: `--file-concurrency`, `--total-concurrency` ✅
- OpenRouter: file_concurrency=3, total_concurrency=100 ✅
- LMStudio: file_concurrency=4, total_concurrency=16 ✅

### 17. Batch statistics ⏳ PARTIAL
- Базовые: количество processed/skipped/failed ✅
- Общее время — не выводится ❌
- Среднее время на файл — не выводится ❌
- Токены/стоимость — tracking есть в status, но не выводится ❌

### 18. Recursive directory support ⏳ TODO
Сейчас только не-рекурсивное сканирование `.md`/`.markdown`.

---

## ❌ Removed from scope (не будет)
- `chat` command — AI агенты вызывают CLI напрямую
- `ask` command — удалена (агенты используют direct LLM)
- `memory` system — возможно вернёмся в Future
- MCP server — AI агенты вызывают CLI как tool
- B2B SaaS — не будет
- CRM интеграции — не будет
- Dashboard/UI — не будет

---

## Architecture Notes

### Workspace + Output Directory (both active)
- Workspace: `~/.media2rag/workspace/<hash>/` — внутренний, checkpoint/resume, версионирование
- Output: `<source>/media2rag-output/<name>/` — пользовательский, читаемый

### Pipeline Checkpoint
Checkpoints в `.pipeline-cache/` внутри workspace документа:
- `cleaned.md` — результат preClean
- `chunks/chunk-NNN.md` — результат split
- `results/result_NNN.json` — результат process (per-chunk)
- `results/results.json` — bulk results

Status также сохраняется в `status.yaml` в output dir (stage, per-chunk done/failed, cost/tokens).

### Pipeline Stages Flow
```
extracted → preClean → splitText → processChunks (parallel)
  → holisticAnalysis → causalExtraction → contextualEnrich (parallel)
  → assemble → pipeline_completed
```

---

## Implementation Plan (actual)

### Phase 1: CLI стабилизация (Now)
- [x] P0 bugs (race, timeout, checkpoint, panic)
- [x] Output directory + resume
- [x] Health check
- [x] File-level concurrency
- [x] Retry logic (pipeline level)
- [x] Progress bar + ETA
- [ ] Robust LLM parsing (multi-line fields)
- [ ] Batch statistics (time, tokens, cost per batch)
- [ ] Recursive directory support
- [ ] Dry-run mode

### Phase 2: RAG + GraphRAG (CLI команды)
- [x] Causal extraction pipeline stage
- [ ] Qdrant restore from git
- [ ] `media2rag rag <query>` — векторный поиск
- [ ] `media2rag graphrag <query>` — обход графа
- [ ] JSON output для AI агентов

### Phase 3: AI Agent Integration
- CLI как tool для Hermes и других агентов
- JSON output format

### Future: Memory (возможно)
- User profile graph
- Персонализация GraphRAG
