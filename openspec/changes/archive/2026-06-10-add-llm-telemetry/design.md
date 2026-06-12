## Context

LLM pipeline делает до **2N+3** вызовов на документ (preClean + process × N + holistic + causal + contextEnrich × N). Ни один из них не логирует токены, стоимость, latency или использованную модель. Инфраструктура для расчёта cost уже есть в `internal/llm/pricing.go` (`CalculateCost`, `ModelPricing`), но она отключена — `ChatResponse` не содержит `Usage`, и `CalculateCost` никем не вызывается.

Полный дизайн с model-схемами, диаграммами и примерами output — в `docs/telemetry.md`.

## Goals / Non-Goals

**Goals:**
- Каждый LLM-вызов автоматически записывает: модель, prompt/completion tokens, prompt/completion chars, latency, cost, успешность, retry attempt
- Два формата: `telemetry.jsonl` (каждый вызов, append-only, crash-safe) и `status.yaml` (агрегаты per-chunk + per-stage + total)
- Минимальные изменения в pipeline-логике — только `WithStage(ctx, "stage_name")` перед вызовом
- Fix `ChunkDone`/`ChunkFailed` — не затирают cost-поля при отметке done

**Non-Goals:**
- Экспорт в OpenTelemetry / Prometheus
- Alert при превышении бюджета
- Dashboard для batch-обработки
- Retroactive-телеметрия для уже обработанных файлов

## Decisions

1. **InstrumentedClient** (wrapper) vs inline instrumentation in pipeline
   - Выбран wrapper: ни один pipeline-метод не меняет логику, только добавляет `WithStage(ctx)`. InstrumentedClient оборачивает `Chat()` и `StreamChat()` — одно место замера и записи.

2. **Context.Context** vs explicit params
   - Выбран context: pipeline прокидывает stage, chunk index, source, retry attempt через context. InstrumentedClient читает их. LLMClient interface не меняется — ни одна реализация не затрагивается.

3. **JSONL** vs JSON array for detailed telemetry
   - Выбран JSONL: append-only, crash-safe, можно анализировать через `jq -s`, не требует полного перечтения при записи.

4. **StatusRecorder агрегирует в PipelineStatus** vs отдельный агрегатор
   - StatusRecorder обновляет `ChunkStatus` (cost/tokens) напрямую при каждом Record(). PipelineStatus.StageBreakdown и Totals считаются инкрементально — без отдельного прохода.

5. **Ollama stream: usage в финальном chunk**
   - Последний SSE-чunk от Ollama содержит `done: true` + `eval_count` / `prompt_eval_count`. StreamChat wrapper аккумулирует контент и вычитывает usage из последнего сообщения.

6. **OpenRouter HTTP retry** — `doWithRetry()` ретраит на 429/503/502/504 внутри OpenRouterClient. Эти ретраи не видны InstrumentedClient (и не должны — токены не тратятся, запрос не дошёл до модели).

## Risks / Trade-offs

- **Concurrent writes** → JSONLRecorder под `sync.Mutex`, StatusRecorder под `PipelineStatus.mu`. Узким местом не станет — запись ~1μs, 3 воркера.
- **Модель из API может не совпасть с pricing key** → `GetPricing()` уже делает fuzzy match через `strings.Contains`. Fallback — нулевая цена (не упадёт, но cost = 0).
- **StreamChat usage не у всех провайдеров** → OpenAI-compatible streaming может не включать `usage`. В таком случае cost = 0, в telemetry пишется с предупреждением.
