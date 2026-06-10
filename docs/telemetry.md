# Telemetry & Cost Tracking — Design

> Tracking every LLM call: tokens, cost, latency, model. Per-chunk in status.yaml, full history in telemetry.jsonl.
> Status: Design (pre-implementation)

---

## Philosophy

Каждый LLM-вызов дорогой. Нужно знать:
- Сколько это стоит ($)
- Сколько токенов уходит (in/out)
- Какая модель реально отвечала (а не только настроенная — fallback может переключить)
- Сколько времени заняло
- Где пробелы: какие стадии самые дорогие, какие чанки ретраятся

Всё это записывается автоматически, без ручного instrument-кода в каждом pipeline-методе.

---

## Pipeline LLM Calls

Всего 5 стадий с LLM:

```
[Pre-Clean]     — 1 вызов (или N parts если текст большой)
  │
[Split]         — без LLM
  │
[Process]       — N вызовов, параллельно (worker pool), до 2 ретраев
[Context Enrich] — N вызовов, параллельно (worker pool)
  │
[Holistic]      — 1 вызов
[Causal]        — 1 вызов
```

Итого: **2N + 3** LLM-вызовов на документ, где N = число чанков.

---

## Data Model

### Usage — встраивается в ChatResponse

```go
// internal/model/llm.go

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
    Model   string  `json:"model"`
    Message Message `json:"message"`
    Usage   *Usage  `json:"usage,omitempty"`   // ← новое
    Done    bool    `json:"done"`
}
```

**Ollama** возвращает `prompt_eval_count` / `eval_count` в `/api/chat`.
**OpenAI-compatible** возвращает `usage.prompt_tokens` / `usage.completion_tokens` в `/v1/chat/completions`.
Оба маппятся в `Usage` при парсинге ответа.

### LLMTelemetry — полная запись одного вызова

```go
// internal/model/telemetry.go

type LLMTelemetry struct {
    Source           string        `json:"source"`            // URL или имя файла
    Stage            string        `json:"stage"`             // "pre_clean" | "process" | "holistic" | "causal" | "context_enrich"
    ChunkIndex       int           `json:"chunk_index,omitempty"`
    RetryAttempt     int           `json:"retry_attempt"`     // 0 = первый вызов, 1 = первый ретрай...
    Model            string        `json:"model"`             // фактическая модель
    PromptTokens     int           `json:"prompt_tokens"`
    CompletionTokens int           `json:"completion_tokens"`
    TotalTokens      int           `json:"total_tokens"`
    PromptChars      int           `json:"prompt_chars"`
    CompletionChars  int           `json:"completion_chars"`
    Cost             float64       `json:"cost"`              // USD
    LatencyMs        int64         `json:"latency_ms"`
    Success          bool          `json:"success"`
    Error            string        `json:"error,omitempty"`   // только если !Success
    Timestamp        time.Time     `json:"timestamp"`
}
```

### ChunkStatus — дополняется cost-полями

```go
// internal/pipeline/status.go

type ChunkStatus struct {
    Index      int     `yaml:"index"`
    Done       bool    `yaml:"done"`
    Failed     bool    `yaml:"failed,omitempty"`
    Error      string  `yaml:"error,omitempty"`
    // NEW:
    Cost       float64 `yaml:"cost,omitempty"`       // стоимость этого чанка (process + context_enrich)
    TokensIn   int     `yaml:"tokens_in,omitempty"`
    TokensOut  int     `yaml:"tokens_out,omitempty"`
    Model      string  `yaml:"model,omitempty"`
    LatencyMs  int64   `yaml:"latency_ms,omitempty"`
}
```

### PipelineStatus — дополняется totals

```go
type PipelineStatus struct {
    Source         string        `yaml:"source"`
    Stage          Stage         `yaml:"stage"`
    ChunksTotal    int           `yaml:"chunks_total"`
    Chunks         []ChunkStatus `yaml:"chunks,omitempty"`
    FailedAt       string        `yaml:"failed_at,omitempty"`
    StartedAt      time.Time     `yaml:"started_at"`
    UpdatedAt      time.Time     `yaml:"updated_at"`
    // NEW:
    TotalCost      float64            `yaml:"total_cost"`
    TotalTokensIn  int                `yaml:"total_tokens_in"`
    TotalTokensOut int                `yaml:"total_tokens_out"`
    StageBreakdown map[string]StageCost `yaml:"stage_breakdown,omitempty"`
}

type StageCost struct {
    Calls   int     `yaml:"calls"`
    Cost    float64 `yaml:"cost"`
    TokensIn  int   `yaml:"tokens_in"`
    TokensOut int  `yaml:"tokens_out"`
}
```

### Пример status.yaml после обработки

```yaml
source: "lecture.md"
stage: done
chunks_total: 12
chunks:
  - index: 1
    done: true
    cost: 0.00042
    tokens_in: 850
    tokens_out: 240
    model: "deepseek/deepseek-chat"
    latency_ms: 2340
  - index: 2
    done: true
    cost: 0.00038
    tokens_in: 720
    tokens_out: 210
    model: "deepseek/deepseek-chat"
    latency_ms: 2100
  # ... остальные чанки
total_cost: 0.00735
total_tokens_in: 14200
total_tokens_out: 3800
stage_breakdown:
  pre_clean:
    calls: 1
    cost: 0.00015
    tokens_in: 320
    tokens_out: 180
  process:
    calls: 12
    cost: 0.00480
    tokens_in: 9600
    tokens_out: 2520
  holistic:
    calls: 1
    cost: 0.00010
    tokens_in: 200
    tokens_out: 80
  causal:
    calls: 1
    cost: 0.00012
    tokens_in: 250
    tokens_out: 90
  context_enrich:
    calls: 12
    cost: 0.00218
    tokens_in: 3830
    tokens_out: 930
started_at: "2026-06-10T10:00:00Z"
updated_at: "2026-06-10T10:05:00Z"
```

---

## Architecture: InstrumentedClient

Главное архитектурное решение — **InstrumentedClient**, обёртка вокруг любого `LLMClient`.

### Context-based metadata

Pipeline прокидывает метаданные через `context.Context`, клиент их читает.

```go
// internal/llm/telemetry.go

type stageKey struct{}
type chunkKey struct{}
type sourceKey struct{}
type retryKey struct{}

func WithStage(ctx context.Context, stage string) context.Context
func WithChunkIndex(ctx context.Context, idx int) context.Context
func WithSource(ctx context.Context, source string) context.Context
func WithRetryAttempt(ctx context.Context, attempt int) context.Context

func stageFromCtx(ctx context.Context) string
func chunkFromCtx(ctx context.Context) int
func sourceFromCtx(ctx context.Context) string
func retryFromCtx(ctx context.Context) int
```

### PromptChars — расчёт из Messages

`ChatRequest` не имеет поля `Prompt` — только `Messages []Message`.
Считаем символы объединением content всех сообщений:

```go
func promptChars(req ChatRequest) int {
    n := 0
    for _, m := range req.Messages {
        n += len(m.Content)
    }
    return n
}
```

### InstrumentedClient

Оборачивает любой `LLMClient`. Перед каждым `Chat()` замеряет время, после — собирает телеметрию.

```go
// internal/llm/instrumented.go

type InstrumentedClient struct {
    inner    LLMClient
    pricing  *PricingStore
    recorder model.TelemetryRecorder
}

func (c *InstrumentedClient) Chat(ctx context.Context, req model.ChatRequest) (*model.ChatResponse, error) {
    start := time.Now()
    resp, err := c.inner.Chat(ctx, req)
    latency := time.Since(start)

    usage := resp.Usage
    if usage == nil {
        usage = &model.Usage{} // fallback для провайдеров без usage
    }

    entry := model.LLMTelemetry{
        Source:           sourceFromCtx(ctx),
        Stage:            stageFromCtx(ctx),
        ChunkIndex:       chunkFromCtx(ctx),
        RetryAttempt:     retryFromCtx(ctx),
        Model:            resp.Model,
        PromptTokens:     usage.PromptTokens,
        CompletionTokens: usage.CompletionTokens,
        TotalTokens:      usage.TotalTokens,
        PromptChars:      promptChars(req),
        CompletionChars:  len(resp.Message.Content),
        Cost:             c.pricing.CalculateCost(resp.Model, usage.PromptTokens, usage.CompletionTokens),
        LatencyMs:        latency.Milliseconds(),
        Success:          err == nil,
        Error:            errMsg(err),
        Timestamp:        time.Now(),
    }

    c.recorder.Record(entry)
    return resp, err
}

### StreamChat — обёртка с накоплением

Стриминг сложнее: token usage приходит только в конце. 
Создаём канал-обёртку, читаем все delta, накапливаем content, ловим финальный `done`.

**Ollama streaming** — последний chunk содержит `done: true` + `eval_count` / `prompt_eval_count`.
**OpenAI streaming** — некоторые провайдеры шлют `usage` в последнем chunk, некоторые нет.

```go
func (c *InstrumentedClient) StreamChat(ctx context.Context, req model.ChatRequest) (<-chan model.StreamDelta, error) {
    innerCh, err := c.inner.StreamChat(ctx, req)
    if err != nil {
        return nil, err
    }

    outCh := make(chan model.StreamDelta)
    go func() {
        start := time.Now()
        var fullContent strings.Builder
        defer close(outCh)

        for delta := range innerCh {
            fullContent.WriteString(delta.Content)
            outCh <- delta
            if delta.Done {
                latency := time.Since(start)
                // Собрать usage из StreamDelta (если есть)
                entry := model.LLMTelemetry{...}
                c.recorder.Record(entry)
            }
        }
    }()
    return outCh, nil
}
```
```

### TelemetryRecorder interface

```go
// internal/model/telemetry.go

type TelemetryRecorder interface {
    Record(entry LLMTelemetry)
    Close() error
}
```

Реализации:

| Recorder | Package | Назначение |
|----------|---------|-----------|
| `JSONLRecorder` | `internal/pipeline/` | Пишет каждую запись в `<output>/telemetry.jsonl` (thread-safe через `sync.Mutex`) |
| `StatusRecorder` | `internal/pipeline/` | Агрегирует в `PipelineStatus` (chunk cost, stage_breakdown, totals) |
| `TeeRecorder` | `internal/model/` | Комбинирует несколько рекордеров |

---

## Pipeline Integration

### Создание InstrumentedClient

В `Pipeline.Run()` или при инициализации:

```go
// pipeline.go
jRecorder := pipeline.NewJSONLRecorder(outputDir, sourceName)
sRecorder := pipeline.NewStatusRecorder(&p.status)

recorder := model.TeeRecorder{jRecorder, sRecorder}

llmClient := llm.NewInstrumentedClient(rawClient, pricingStore, recorder)
p.llmClient = llmClient
```

### Изменения в каждом pipeline-методе

Только добавление `WithStage()` — логика не меняется:

**preClean:**
```go
ctx = llm.WithStage(ctx, "pre_clean")
ctx = llm.WithSource(ctx, p.source)
result, err := p.llmClient.Chat(ctx, req)
```
Если текст большой — каждая часть чистится отдельным вызовом с `WithRetryAttempt(i)`.

**processSingle (per chunk):**
```go
ctx = llm.WithStage(ctx, "process")
ctx = llm.WithChunkIndex(ctx, index)
// retry loop:
for attempt := 0; attempt <= maxRetries; attempt++ {
    retryCtx := llm.WithRetryAttempt(ctx, attempt)
    result, err := p.llmClient.Chat(retryCtx, req)
}
```

**contextualEnrich (per chunk):**
```go
ctx = llm.WithStage(ctx, "context_enrich")
ctx = llm.WithChunkIndex(ctx, index)
result, err := p.llmClient.Chat(ctx, req)
```

**holisticAnalysis:**
```go
ctx = llm.WithStage(ctx, "holistic")
result, err := p.llmClient.Chat(ctx, req)
```

**causalExtraction:**
```go
ctx = llm.WithStage(ctx, "causal")
result, err := p.llmClient.Chat(ctx, req)
```

---

## Provider Response Parsing

### Ollama `/api/chat` response

```json
{
  "model": "qwen3.5:27b",
  "message": {"role": "assistant", "content": "..."},
  "prompt_eval_count": 450,
  "eval_count": 120,
  "done": true
}
```

Ollama не использует `usage` object — поля на верхнем уровне. При парсинге маппим:

```go
prompt_eval_count → Usage.PromptTokens
eval_count       → Usage.CompletionTokens
prompt_eval_count + eval_count → Usage.TotalTokens
```

### OpenAI-compatible `/v1/chat/completions` response

```json
{
  "model": "deepseek/deepseek-chat",
  "choices": [{"message": {"content": "..."}}],
  "usage": {
    "prompt_tokens": 450,
    "completion_tokens": 120,
    "total_tokens": 570
  }
}
```

Прямой маппинг в `Usage`.

### Pricing

`CalculateCost(model, promptTokens, completionTokens)` уже существует в `internal/llm/pricing.go`.
Цены загружаются из `models.dev/api/json` с кэшем 15 минут, fallback на хардкод.

---

## File Formats

### telemetry.jsonl

**Одна JSON-строка на LLM-вызов**, append-only. Не ломается при креше.

```jsonl
{"source":"lecture.md","stage":"process","chunk_index":1,"retry_attempt":0,"model":"deepseek/deepseek-chat","prompt_tokens":450,"completion_tokens":120,"total_tokens":570,"prompt_chars":3200,"completion_chars":850,"cost":0.00042,"latency_ms":2340,"success":true,"timestamp":"2026-06-10T10:01:00Z"}
{"source":"lecture.md","stage":"process","chunk_index":2,"retry_attempt":0,"model":"deepseek/deepseek-chat","prompt_tokens":420,"completion_tokens":110,"total_tokens":530,"prompt_chars":2900,"completion_chars":780,"cost":0.00038,"latency_ms":2100,"success":true,"timestamp":"2026-06-10T10:01:02Z"}
```

Можно анализировать через CLI:
```bash
# Общая стоимость
cat telemetry.jsonl | jq -s 'map(.cost) | add'

# Средняя latency по стадиям
cat telemetry.jsonl | jq -s 'group_by(.stage) | map({stage: .[0].stage, avg_latency: (map(.latency_ms) | add / length)})'

# Самые дорогие чанки
cat telemetry.jsonl | jq -s 'sort_by(.cost) | reverse[:5]'

# Ретраи (error)
cat telemetry.jsonl | jq 'select(.success == false)'
```

### status.yaml

Дополняется cost-полями (см. Data Model выше). Сохраняется при каждом обновлении через `Save()`.

### telemetry.jsonl vs status.yaml

| Аспект | telemetry.jsonl | status.yaml |
|--------|----------------|-------------|
| Формат | JSONL (append) | YAML (rewrite) |
| Детализация | Каждый LLM-вызов | Агрегаты + total |
| Размер | Большой (N записей) | Маленький |
| Анализ | jq, grep, awk | Быстрый взгляд |
| Crash-safe | Да (append) | Нет (rewrite) |

---

## Fix: ChunkDone не затирает cost-поля

Текущий `ChunkDone()` (status.go:98) перезаписывает весь `ChunkStatus`, уничтожая cost/tokens/latency.

**Фикс — сохранять существующие поля:**

```go
func (s *PipelineStatus) ChunkDone(index int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if index >= 0 && index < len(s.Chunks) {
        existing := s.Chunks[index]
        existing.Done = true
        s.Chunks[index] = existing
    }
    s.UpdatedAt = time.Now()
    s.Save()
}
```

Тот же фикс применить к `ChunkFailed` и `SetChunks` — обе перезаписывают.

## Files to Change

```
internal/
  model/
    llm.go              ← + Usage struct, + Usage field в ChatResponse
    telemetry.go        ← NEW: LLMTelemetry, TelemetryRecorder interface, TeeRecorder
  llm/
    client.go           ← без изменений (interface не трогаем)
    ollama.go           ← парсинг prompt_eval_count/eval_count в Usage
    openrouter.go       ← парсинг usage в Usage
    instrumented.go     ← NEW: InstrumentedClient
    telemetry.go        ← NEW: context keys (WithStage, WithSource, etc.)
    resolver.go         ← возможно обёртка InstrumentedClient при создании клиента
  pipeline/
    status.go           ← + cost-поля в ChunkStatus, PipelineStatus, + StageCost
    telemetry.go        ← NEW: JSONLRecorder, StatusRecorder
    pipeline.go         ← + WithStage() в каждом LLM-вызове
    processor.go        ← + WithStage() + WithChunkIndex + WithRetryAttempt
    assembler.go        ← без изменений
  events/
    emitter.go          ← без изменений
cmd/
  media2rag/
    process.go          ← создать JSONLRecorder и StatusRecorder, передать в pipeline
```

---

## Implementation Order

1. **Model** — `Usage` в `llm.go`, `LLMTelemetry` + `TelemetryRecorder` в `telemetry.go`
2. **Provider parsing** — Ollama и OpenRouter: парсинг token usage из ответов
3. **Context keys** — `WithStage`, `WithChunkIndex`, `WithSource`, `WithRetryAttempt`
4. **InstrumentedClient** — обёртка с замером времени и записью телеметрии
5. **Recorders** — `JSONLRecorder` (пишет .jsonl), `StatusRecorder` (агрегирует в status)
6. **Pipeline integration** — `WithStage()` в 5 местах, `WithChunkIndex` в process+enrich
7. **CLI integration** — создание рекордеров в `process.go`
8. **pricing.go fixes** — `CalculateCost` вызывается, `ChatResponse.Model` используется для lookup

---

## Future (not in scope)

- Экспорт телеметрии в OpenTelemetry / Prometheus
- Alert при превышении бюджета (total_cost > threshold)
- Dashboard для batch-обработки (fastHTML + htmx)
