## Why

Каждый LLM-вызов стоит денег, но сейчас pipeline не отслеживает ни токены, ни стоимость. Невозможно понять, сколько реально уходит на обработку файла, какие стадии самые дорогие, какие модели выгоднее. Без телеметрии — управление вслепую.

## What Changes

- **New data model**: `Usage` в `ChatResponse`, `LLMTelemetry` для каждого вызова, cost-поля в `ChunkStatus`/`PipelineStatus`
- **InstrumentedClient**: прозрачная обёртка вокруг `LLMClient`, измеряет latency, парсит token usage из ответа провайдера, считает cost через `CalculateCost()`
- **Context-based metadata**: pipeline прокидывает stage/chunk index через `context.Context` — клиент не зависит от pipeline
- **Два формата вывода**: `telemetry.jsonl` (каждый вызов, crash-safe append) + `status.yaml` (агрегаты: total cost, stage breakdown, per-chunk cost)
- **5 pipeline-интеграций**: `WithStage()` перед каждым LLM-вызовом (preClean, process, holistic, causal, contextEnrich) без изменения логики
- **Фикс `ChunkDone`**: перестаёт затирать cost-поля при отметке done

## Capabilities

### New Capabilities
- `llm-telemetry`: tracking каждого LLM-вызова — токены, стоимость, latency, модель, per-stage breakdown

### Modified Capabilities
<!-- No spec-level requirement changes — existing contracts unchanged -->

## Impact

- **8 файлов изменяются**, 4 новых файла создаются
- `internal/llm/`: новый `instrumented.go` + `telemetry.go` (context keys)
- `internal/model/`: `Usage` в `ChatResponse`, новый `telemetry.go` (LLMTelemetry + интерфейсы)
- `internal/pipeline/`: `status.go` (cost-поля), новый `telemetry.go` (JSONLRecorder + StatusRecorder), `pipeline.go` + `processor.go` (WithStage)
- `cmd/media2rag/process.go`: создание рекордеров
- Никаких внешних зависимостей, никаких брейкинг-ченджей
