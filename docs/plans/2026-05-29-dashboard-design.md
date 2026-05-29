# Dashboard Design — media2rag Observability

Date: 2026-05-29
Status: Draft

## 1. Philosophy

This dashboard follows industry best practices from LangSmith, Arize Phoenix, and Datadog LLM Observability. Every component exists to answer one question: **"Is my RAG system improving or regressing?"**

Three pillars:
- **Traceability** — every pipeline run, every LLM call, every stage is captured
- **Evaluation** — LLM-as-judge + human feedback + automated metrics
- **Iteration** — regression detection, comparison, drill-down to root cause

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────┐
│               Dashboard UI (Svelte SPA)                  │
│  /           Overview (drill-down cards + timeline)      │
│  /pipeline   Pipeline Runs (list + detail)               │
│  /logs       LLM Call Logs (filterable)                  │
│  /metrics    Aggregated Metrics (time-series)            │
│  /embeddings Embedding Validation                        │
│  /feedback   Human Feedback (like/dislike, ratings)      │
│  /regressions Regression Comparison (before/after)       │
│  /debug      System Status + Live SSE Monitor            │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTP/JSON (+ SSE for live)
┌──────────────────────▼──────────────────────────────────┐
│               Go API (internal/api/debug.go)             │
│                                                          │
│  GET  /api/debug/overview       — aggregated KPIs        │
│  GET  /api/debug/timeline       — score timeline         │
│  GET  /api/debug/pipeline       — list pipeline runs     │
│  GET  /api/debug/pipeline/{id}  — single run detail      │
│  GET  /api/debug/logs           — list LLM calls         │
│  GET  /api/debug/logs/{id}      — single LLM call        │
│  GET  /api/debug/metrics        — aggregated metrics     │
│  GET  /api/debug/embeddings     — embedding checks       │
│  GET  /api/debug/feedback       — feedback list          │
│  POST /api/debug/feedback       — submit feedback        │
│  GET  /api/debug/regressions    — regression comparison  │
│  GET  /api/debug/status         — system component status│
│  GET  /api/debug/config         — current configuration  │
│  POST /api/debug/reprocess/{id} — re-run pipeline        │
│  GET  /api/debug/live           — SSE event stream       │
└──────────────┬──────────────────────────────┬───────────┘
               │                              │
    ┌──────────▼──────────┐       ┌───────────▼───────────┐
    │     SQLite          │       │       Qdrant          │
    │  (operational DB)   │       │  (vector validation)  │
    │                     │       │                       │
    │ pipeline_runs       │       │ collections:          │
    │ pipeline_stages     │       │  - chunks             │
    │ llm_calls           │       │  - queries_log        │
    │ judge_evaluations   │       │                       │
    │ embedding_checks    │       │ Used for:             │
    │ feedback            │       │  - similarity search  │
    │ metrics_cache       │       │  - relevance checks   │
    │ prompt_versions     │       │  - regression compare │
    └─────────────────────┘       └───────────────────────┘
```

### 2.1 Data Flow

1. Pipeline starts → creates `pipeline_runs` row
2. Each stage (compress/process/assemble) → creates `pipeline_stages` row with prompt, response, score, tokens, latency
3. Each LLM call → creates `llm_calls` row with model, tokens, latency, cost
4. After pipeline → optional LLM-as-judge runs → creates `judge_evaluations` row
5. Periodic embedding check → queries Qdrant → creates `embedding_checks` row
6. User can submit feedback → creates `feedback` row
7. Metrics cache recomputed on schedule or on-demand
8. All changes emitted via SSE for live dashboard updates

### 2.2 SSE Event Types

| Event | Data | Trigger |
|-------|------|---------|
| `pipeline_start` | `{ run_id, source }` | Pipeline started |
| `pipeline_complete` | `{ run_id, score, latency_ms }` | Pipeline finished |
| `stage_complete` | `{ run_id, stage, score, latency }` | Each stage done |
| `llm_call` | `{ call_id, model, tokens, latency }` | Each LLM call |
| `judge_complete` | `{ run_id, score, reason }` | Judge evaluation done |
| `feedback` | `{ run_id, rating }` | New feedback submitted |
| `heartbeat` | `{ ts }` | Every 30s keepalive |

---

## 3. Data Model (SQLite)

### 3.1 pipeline_runs

```sql
CREATE TABLE pipeline_runs (
    id            TEXT PRIMARY KEY,          -- uuid
    source        TEXT NOT NULL,             -- URL or file path
    source_type   TEXT DEFAULT '',           -- video, article, pdf, audio, markdown
    status        TEXT NOT NULL DEFAULT 'running',  -- running, completed, failed
    score         REAL DEFAULT 0.0,          -- aggregate score 0.0-1.0
    total_tokens  INTEGER DEFAULT 0,
    total_latency_ms REAL DEFAULT 0.0,
    total_cost    REAL DEFAULT 0.0,
    error         TEXT DEFAULT '',           -- error message if failed
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at  TEXT,
    -- metadata
    pipeline_version TEXT DEFAULT '',        -- for regression comparison
    config_snapshot TEXT DEFAULT ''          -- JSON of config at time of run
);

CREATE INDEX idx_pipeline_runs_created ON pipeline_runs(created_at DESC);
CREATE INDEX idx_pipeline_runs_score ON pipeline_runs(score DESC);
CREATE INDEX idx_pipeline_runs_status ON pipeline_runs(status);
```

### 3.2 pipeline_stages

```sql
CREATE TABLE pipeline_stages (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,             -- compress, process, assemble, judge
    seq           INTEGER DEFAULT 0,        -- stage order
    status        TEXT NOT NULL DEFAULT 'running',
    score         REAL DEFAULT 0.0,
    latency_ms    REAL DEFAULT 0.0,
    tokens_in     INTEGER DEFAULT 0,
    tokens_out    INTEGER DEFAULT 0,
    prompt        TEXT DEFAULT '',           -- full prompt sent
    response      TEXT DEFAULT '',           -- full response received
    model         TEXT DEFAULT '',           -- which model was used
    error         TEXT DEFAULT '',
    started_at    TEXT,
    completed_at  TEXT
);

CREATE INDEX idx_pipeline_stages_run ON pipeline_stages(run_id);
```

### 3.3 llm_calls

```sql
CREATE TABLE llm_calls (
    id               TEXT PRIMARY KEY,
    run_id           TEXT REFERENCES pipeline_runs(id) ON DELETE SET NULL,
    stage_name       TEXT DEFAULT '',        -- which stage triggered this call
    model            TEXT NOT NULL,          -- e.g. qwen/qwen3-coder:free
    operation        TEXT NOT NULL,          -- judge, compress, process, assemble, embed, query
    provider         TEXT DEFAULT 'openrouter', -- openrouter, ollama, openai
    prompt_tokens    INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    latency_ms       REAL DEFAULT 0.0,
    cost             REAL DEFAULT 0.0,       -- USD
    prompt           TEXT DEFAULT '',
    response         TEXT DEFAULT '',
    status           TEXT DEFAULT 'ok',      -- ok, error, timeout
    error_message    TEXT DEFAULT '',
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_llm_calls_run ON llm_calls(run_id);
CREATE INDEX idx_llm_calls_model ON llm_calls(model);
CREATE INDEX idx_llm_calls_created ON llm_calls(created_at DESC);
CREATE INDEX idx_llm_calls_operation ON llm_calls(operation);
```

### 3.4 judge_evaluations

```sql
CREATE TABLE judge_evaluations (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    judge_model   TEXT NOT NULL,             -- which model acted as judge
    judge_type    TEXT NOT NULL DEFAULT 'quality',  -- quality, relevance, faithfulness, helpfulness
    prompt        TEXT NOT NULL,             -- the judge prompt (includes context + answer)
    response      TEXT NOT NULL,             -- the judge's evaluation response
    score         REAL DEFAULT 0.0,          -- parsed score from judge 0.0-1.0
    reasoning     TEXT DEFAULT '',           -- judge's reasoning/explanation
    passed        INTEGER DEFAULT 0,        -- boolean: passed threshold?
    latency_ms    REAL DEFAULT 0.0,
    tokens_used   INTEGER DEFAULT 0,
    cost          REAL DEFAULT 0.0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_judge_run ON judge_evaluations(run_id);
CREATE INDEX idx_judge_score ON judge_evaluations(score DESC);
```

### 3.5 embedding_checks

```sql
CREATE TABLE embedding_checks (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    query_text    TEXT NOT NULL,             -- the search query used
    chunk_text    TEXT NOT NULL,             -- the retrieved chunk
    chunk_id      TEXT DEFAULT '',           -- reference to chunk in Qdrant
    similarity_score REAL DEFAULT 0.0,       -- cosine similarity in Qdrant
    relevance_score   REAL DEFAULT 0.0,      -- LLM-judged relevance 0.0-1.0
    expected_relevance TEXT DEFAULT '',      -- what the chunk should contain
    passed        INTEGER DEFAULT 0,        -- boolean
    latency_ms    REAL DEFAULT 0.0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_embedding_run ON embedding_checks(run_id);
CREATE INDEX idx_embedding_similarity ON embedding_checks(similarity_score DESC);
```

### 3.6 feedback

```sql
CREATE TABLE feedback (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    llm_call_id   TEXT REFERENCES llm_calls(id) ON DELETE SET NULL,
    rating        INTEGER DEFAULT 3,         -- 1-5 scale
    like_dislike  TEXT DEFAULT '',           -- 'like', 'dislike', ''
    category      TEXT DEFAULT '',           -- 'accurate', 'hallucination', 'irrelevant', 'incomplete', 'other'
    comment       TEXT DEFAULT '',
    source        TEXT DEFAULT 'dashboard',  -- dashboard, api, sdk
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_feedback_run ON feedback(run_id);
CREATE INDEX idx_feedback_rating ON feedback(rating);
```

### 3.7 metrics_cache

```sql
CREATE TABLE metrics_cache (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key     TEXT NOT NULL UNIQUE,      -- e.g. 'overview', 'timeline:7d', 'metrics:30d'
    data          TEXT NOT NULL,             -- JSON blob
    computed_at   TEXT NOT NULL DEFAULT (datetime('now')),
    ttl_seconds   INTEGER DEFAULT 300       -- 5min default TTL
);

CREATE INDEX idx_metrics_key ON metrics_cache(cache_key);
```

### 3.8 prompt_versions

```sql
CREATE TABLE prompt_versions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT NOT NULL,             -- 'judge_quality', 'compress', 'process', 'assemble'
    version       INTEGER NOT NULL,
    prompt_text   TEXT NOT NULL,
    model         TEXT DEFAULT '',
    parameters    TEXT DEFAULT '{}',         -- JSON: temperature, max_tokens, etc.
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    notes         TEXT DEFAULT '',
    UNIQUE(name, version)
);

CREATE INDEX idx_prompt_name ON prompt_versions(name, version DESC);
```

---

## 4. LLM Model Strategy

### 4.1 Model Tiers

| Role | Primary (Free) | Fallback (Cheap Paid) | Rationale |
|------|---------------|----------------------|-----------|
| **Pipeline** (compress/process/assemble) | `openrouter/free` or `deepseek/deepseek-v4-flash:free` | `mistralai/mistral-nemo` ($0.02/$0.03) | Fast, tool-calling, 1M ctx |
| **Judge** (quality evaluation) | `nvidia/nemotron-3-super-120b-a12b:free` | `qwen/qwen3.5-9b` ($0.04/$0.15) | Strong reasoning needed |
| **Judge Fallback** | `qwen/qwen3-coder:free` | `meta-llama/llama-3.1-8b-instruct` ($0.02/$0.05) | If primary judge fails |
| **Embedding** | Local Ollama (`nomic-embed-text`) | `qwen/qwen3-embedding-4b` via inference ($0.01/M) | Always local if available |
| **Embedding Check** | Same as pipeline | — | Reuse embedding model |

### 4.2 Fallback Chain

```
Pipeline LLM:
  1. Try openrouter/free (routes to best free)
  2. On error → try deepseek/deepseek-v4-flash:free
  3. On error → try mistralai/mistral-nemo ($0.02/$0.03)
  4. On error → try meta-llama/llama-3.2-3b-instruct:free
  5. On error → return error with details

Judge LLM:
  1. Try nvidia/nemotron-3-super-120b-a12b:free
  2. On error → try qwen/qwen3-coder:free
  3. On error → try qwen/qwen3.5-9b ($0.04/$0.15)
  4. On error → skip judge, log warning
```

### 4.3 Cost Tracking

Every `llm_calls` row stores `cost` in USD, computed as:
```
cost = (prompt_tokens / 1_000_000 * input_price) +
       (completion_tokens / 1_000_000 * output_price)
```

Prices pulled from models.dev API (`https://models.dev/api.json`), cached locally in config.

---

## 5. Pipeline Tracing

### 5.1 Tracing Hook

In the pipeline code, every stage wraps its execution:

```go
type TraceContext struct {
    RunID    string
    Stage    string
    Prompt   string
    Response string
    TokensIn int
    TokensOut int
    Latency  time.Duration
    Score    float64
    Error    error
}

// Inserted at pipeline.go: each stage call
func (p *Pipeline) runStage(ctx context.Context, name string, fn StageFunc) error {
    start := time.Now()
    result, err := fn(ctx)
    latency := time.Since(start)

    p.tracer.SaveStage(TraceContext{
        RunID:    p.runID,
        Stage:    name,
        Prompt:   result.Prompt,
        Response: result.Response,
        TokensIn: result.TokensIn,
        TokensOut: result.TokensOut,
        Latency:  latency,
        Score:    result.Score,
        Error:    err,
    })
    return err
}
```

### 5.2 Data Captured Per Pipeline Run

| Field | Source | Example |
|-------|--------|---------|
| source | user input | `https://example.com/article` |
| source_type | extractor | `article` |
| status | pipeline result | `completed` |
| score | judge evaluation | `0.87` |
| total_tokens | sum of all stages | `15234` |
| total_latency_ms | sum of all stages | `12340.5` |
| total_cost | sum of all LLM calls | `0.0015` |
| error | first error encountered | `""` |
| pipeline_version | git commit hash | `abc1234` |
| config_snapshot | serialized config | JSON |

### 5.3 Data Captured Per Stage

| Field | Source | Example |
|-------|--------|---------|
| name | stage name | `compress` |
| seq | stage order | `1` |
| score | stage score | `0.95` |
| latency_ms | stage duration | `2340.5` |
| tokens_in | input tokens | `8000` |
| tokens_out | output tokens | `1200` |
| prompt | full prompt sent | `"Compress this text..."` |
| response | full response | `"Compressed version..."` |
| model | model used | `deepseek/deepseek-v4-flash:free` |
| error | stage error | `""` |

### 5.4 Data Captured Per LLM Call

| Field | Source | Example |
|-------|--------|---------|
| run_id | parent run | `uuid` |
| stage_name | calling stage | `compress` |
| model | model used | `qwen/qwen3-coder:free` |
| operation | call type | `compress` |
| prompt_tokens | token count | `4500` |
| completion_tokens | token count | `800` |
| latency_ms | call duration | `1230.5` |
| cost | computed USD | `0.0000` |
| prompt | request body | `[...]` |
| response | response body | `[...]` |
| status | success/fail | `ok` |
| error_message | if failed | `"timeout"` |

---

## 6. LLM-as-Judge Evaluation

### 6.1 Judge Types

| Judge Type | What It Evaluates | Prompt Template |
|-----------|------------------|----------------|
| `quality` | Overall answer quality, correctness, completeness | `You are evaluating a RAG system. The user asked: {question}. The system responded: {answer}. Evaluate on: accuracy, completeness, clarity. Score 0-10.` |
| `relevance` | Whether the answer is relevant to the question | `Does the following answer directly address the user's question? Question: {q}. Answer: {a}. Score 0-10.` |
| `faithfulness` | Whether the answer is grounded in the context | `Does the answer contain information not present in the provided context? Context: {ctx}. Answer: {a}. Score 0-10.` |
| `helpfulness` | How helpful the answer is | `How helpful is this answer? Consider: does it solve the user's need? Score 0-10.` |

### 6.2 Judge Pipeline

```
Pipeline completes
  → Extract: question, answer, context chunks
  → Build judge prompt with rubric
  → Call judge LLM (nemotron-3-super-120b:free)
  → Parse score from response (0-10 → 0.0-1.0)
  → Store in judge_evaluations table
  → Update pipeline_runs.score with weighted average
  → Emit SSE: judge_complete
```

### 6.3 Score Parsing

The judge response is parsed for a score pattern:
```
Score: 8/10  → 0.8
Rating: 7     → 0.7
{ "score": 9 } → 0.9
```

Fallback: if no score found, use LLM to extract score or default to 0.5.

### 6.4 Weighted Average

```
pipeline_runs.score = (
    quality_score * 0.4 +
    relevance_score * 0.3 +
    faithfulness_score * 0.2 +
    helpfulness_score * 0.1
)
```

---

## 7. Embedding Validation

### 7.1 How It Works

After each pipeline run, a sample of chunks is validated:

1. Take the user's query (or generate a test query from content)
2. Search Qdrant with the query
3. For each top-5 returned chunk:
   - Record `similarity_score` from Qdrant
   - Send (query + chunk) to an LLM for relevance scoring
   - Store result in `embedding_checks`
4. Compute aggregate: `avg_similarity`, `avg_relevance`, `pass_rate`

### 7.2 Embedding Check Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `sample_size` | 5 | Number of chunks to check per run |
| `relevance_threshold` | 0.7 | Minimum LLM-judged relevance |
| `similarity_threshold` | 0.6 | Minimum cosine similarity |
| `check_frequency` | every N runs | 1 = every run, 10 = every 10th run |

### 7.3 Dashboard Visualizations

- **Similarity Distribution**: histogram of cosine similarity scores
- **Relevance vs Similarity**: scatter plot (similarity on x, relevance on y)
- **Failed Embeddings**: list of chunks where relevance < threshold
- **Trend**: average similarity/relevance over time

---

## 8. Feedback System

### 8.1 Feedback Types

| Type | Input | UI Element |
|------|-------|-----------|
| **Like/Dislike** | Binary (thumbs up/down) | Buttons next to each response |
| **Rating** | 1-5 stars | Star selector |
| **Category** | Enum: accurate, hallucination, irrelevant, incomplete, other | Tag selector |
| **Comment** | Free text | Text area |

### 8.2 API

```json
POST /api/debug/feedback
{
    "run_id": "uuid",
    "llm_call_id": "uuid (optional)",
    "rating": 4,
    "like_dislike": "like",
    "category": "accurate",
    "comment": "Great answer, very detailed"
}

Response: { "id": "uuid", "status": "created" }
```

### 8.3 Impact of Feedback

Feedback affects:
- `pipeline_runs.score`: if dislike, score is penalized
- `metrics_cache.overview`: pass rate recalculated
- Regression detection: runs with negative feedback flagged

---

## 9. Regression Detection

### 9.1 How It Works

Compare recent pipeline runs against a baseline period:

```sql
-- Current period: last 24h
-- Baseline period: 24h-48h ago

SELECT
    AVG(score) as avg_score,
    COUNT(*) as total_runs,
    SUM(CASE WHEN score >= 0.7 THEN 1 ELSE 0 END) * 1.0 / COUNT(*) as pass_rate
FROM pipeline_runs
WHERE created_at BETWEEN ? AND ?
```

### 9.2 Regression Signals

| Signal | Check | Alert Threshold |
|--------|-------|----------------|
| Score drop | Current avg < baseline avg - 0.1 | 🔴 Major |
| Pass rate drop | Current < baseline - 5% | 🟡 Minor |
| Latency increase | Current > baseline * 1.5 | 🟡 Minor |
| Token increase | Current > baseline * 1.3 | 🟡 Minor |
| Error rate | Current > baseline + 2% | 🔴 Major |
| Embedding quality drop | Current relevance < baseline - 0.1 | 🔴 Major |
| Negative feedback ratio | Current > baseline * 2 | 🟡 Minor |

### 9.3 API

```json
GET /api/debug/regressions?period=24h&baseline=48h

{
    "current_period": { "start": "...", "end": "..." },
    "baseline_period": { "start": "...", "end": "..." },
    "signals": [
        {
            "name": "avg_score",
            "current": 0.82,
            "baseline": 0.89,
            "delta": -0.07,
            "status": "minor",
            "direction": "down"
        },
        {
            "name": "pass_rate",
            "current": 0.88,
            "baseline": 0.94,
            "delta": -0.06,
            "status": "minor",
            "direction": "down"
        }
    ],
    "top_regressions": [
        { "run_id": "uuid", "source": "...", "score": 0.3, "prev_similar_run_score": 0.85 }
    ]
}
```

---

## 10. Dashboard UI Pages

### 10.1 Overview (`/`)

**Layout:**
```
┌──────────────────────────────────────────────────────────┐
│  Overview                                          [7d] │
├───────────┬───────────┬───────────┬───────────┬─────────┤
│ Score     │ Pass Rate │ Documents │ Avg       │ Cost    │
│ 85        │ 92%       │ 143       │ Latency   │ $0.42   │
│ ▲ +2 wk   │           │           │ 2.3s      │         │
├───────────┴───────────┴───────────┴───────────┴─────────┤
│ Score Radar (bar chart: quality, relevance, faithfulness)│
│ Timeline (7d line chart: avg score per day)             │
├───────────┬─────────────────────────────────────────────┤
│ Top       │ Latest Pipeline Run                          │
│ Failures  │ 📄 source                                    │
│ 1. url    │ compress: ✓ 0.92                             │
│ 2. url    │ process:  ✓ 0.88                             │
│ 3. url    │ assemble: ✓ 0.95                             │
│           │ judge:     ✓ 0.87                            │
├───────────┴─────────────────────────────────────────────┤
│ Quick Actions: Recent Feedback, Recent Regressions       │
└──────────────────────────────────────────────────────────┘
```

**Components:**
- `KPICard` — big number, label, trend arrow
- `ScoreRadar` — horizontal bar chart per evaluation type
- `TimelineChart` — SVG line chart (7/14/30 days)
- `FailuresList` — clickable rows → `/pipeline/{id}`
- `LatestPipeline` — stage scores with color coding
- `PeriodSelector` — 24h, 7d, 14d, 30d

### 10.2 Pipeline Runs (`/pipeline`)

**Layout:**
```
┌──────────────────────────────────────────────────────────┐
│ Pipeline Runs                              [24h ▼] [▼]  │
├──────────────────────────────────────────────────────────┤
│ Table:                                                    │
│ Run ID     | Source          | Score | Tokens | Latency  │
│ uuid       | https://...     | 0.85  | 12.3k  | 2.3s    │
│ ...                                                      │
├──────────────────────────────────────────────────────────┤
│ Click row → expanded detail:                              │
│  ├─ Stages: compress ✓ | process ✓ | assemble ✓ | judge ✓│
│  ├─ Stage expand → prompt + response                     │
│  ├─ Judge details: model, score, reasoning                │
│  ├─ Embedding check: similarity, relevance                │
│  └─ Feedback: ★★★★☆ "Great"                              │
└──────────────────────────────────────────────────────────┘
```

**Filters:**
- Time range: 24h, 7d, 30d, all
- Status: all, completed, failed
- Score: all, low (<0.5), medium (0.5-0.8), high (>0.8)
- Source type: all, video, article, pdf, audio, markdown

**Row actions:**
- 👁 View detail → `/pipeline/{id}`
- 🔄 Reprocess → `POST /api/debug/reprocess/{id}`
- 👍👎 Quick feedback

### 10.3 Pipeline Detail (`/pipeline/{id}`)

Full single-run view:
```
┌──────────────────────────────────────────────────────────┐
│ ← Back to list                                           │
│ Pipeline: uuid                        Score: 0.85        │
│ Source: https://example.com  (article)   👍 👎           │
├──────────────────────────────────────────────────────────┤
│ Summary: Tokens 12,340 | Latency 2.3s | Cost $0.0015    │
│ Pipeline version: abc1234 | Created: 2026-05-29 14:30   │
├──────────────────────────────────────────────────────────┤
│ Stages Timeline (horizontal bar per stage with timing)   │
│                                                          │
│ compress  ██████████████████████ 0.95  2.3s  4500t      │
│ process   ██████████████████████████ 0.88  5.1s  8000t  │
│ assemble  ████████████████████ 0.92  1.2s  2000t        │
│ judge     █████████████████████ 0.85  3.7s  3400t       │
├──────────────────────────────────────────────────────────┤
│ Expand any stage → full prompt/response                  │
├──────────────────────────────────────────────────────────┤
│ LLM Calls (table):                                       │
│ Time   | Model          | Op      | Tokens | Latency    │
│ 14:30  | deepseek:free  | compress | 4500  | 1230ms     │
│ ...                                                      │
├──────────────────────────────────────────────────────────┤
│ Judge Evaluation:                                         │
│   Model: nemotron-3-super-120b:free                      │
│   Quality: 0.85 — "The answer is accurate..."            │
│   Relevance: 0.90 — "Directly addresses the question"    │
│   Faithfulness: 0.80 — "Mostly grounded in context"      │
│   [view judge prompt] [view judge response]              │
├──────────────────────────────────────────────────────────┤
│ Embedding Check:                                          │
│   Avg Similarity: 0.78 | Avg Relevance: 0.82             │
│   ├─ Chunk 1: sim 0.85 rel 0.90 ✅                      │
│   ├─ Chunk 2: sim 0.72 rel 0.65 ⚠️                      │
│   └─ Chunk 3: sim 0.62 rel 0.45 ❌ (relevance < 0.7)    │
├──────────────────────────────────────────────────────────┤
│ Feedback:                                                 │
│   Rating: 4/5 | Like | Category: accurate                │
│   Comment: "Very helpful, but could be shorter"          │
│   [submit feedback]                                      │
└──────────────────────────────────────────────────────────┘
```

### 10.4 LLM Logs (`/logs`)

```
┌──────────────────────────────────────────────────────────┐
│ LLM Call Logs                              [Filters]     │
├──────────────────────────────────────────────────────────┤
│ Table:                                                    │
│ Time     | Model              | Op      | Tkn | Score    │
│ 14:30:12 | deepseek:free      | compress | 4.5k | ✅     │
│ 14:30:15 | qwen3-coder:free   | judge   | 3.4k | 0.85   │
│ 14:31:00 | mistral-nemo       | process | 8.0k | ✅     │
│ ...                                                       │
├──────────────────────────────────────────────────────────┤
│ Expand row → full prompt/response, latency, cost          │
├──────────────────────────────────────────────────────────┤
│ Filters (all select dropdowns):                           │
│ Model: [all ▼] Operation: [all ▼] Status: [all ▼]       │
│ Time: [24h ▼]                                             │
└──────────────────────────────────────────────────────────┘
```

### 10.5 Metrics (`/metrics`)

```
┌──────────────────────────────────────────────────────────┐
│ Metrics                                          [7d ▼] │
├──────────────────────────────────────────────────────────┤
│ Score Distribution (histogram)                           │
│   0.0-0.2: ██ 2                                           │
│   0.2-0.4: ████ 4                                         │
│   0.4-0.6: ████████ 8                                     │
│   0.6-0.8: ██████████████████ 18                          │
│   0.8-1.0: ████████████████████████████ 28                │
├──────────────────────────────────────────────────────────┤
│ Model Comparison (horizontal bars)                        │
│ deepseek:free    ████████████████████████ 0.85            │
│ qwen3-coder:free ██████████████████ 0.72                  │
│ mistral-nemo     ██████████████████████████ 0.88          │
├──────────────────────────────────────────────────────────┤
│ Score by Source Type                                      │
│ article: 0.82  ████████████████████████                   │
│ video:   0.76  ██████████████████████                     │
│ pdf:     0.79  ███████████████████████                    │
│ audio:   0.71  ██████████████████                         │
├──────────────────────────────────────────────────────────┤
│ Token Usage: Today 45.2k | Week 312k | Month 1.2M        │
│ Cost:       Today $0.00 | Week $0.01 | Month $0.05       │
│ (blank if free models only)                               │
├──────────────────────────────────────────────────────────┤
│ Latency P50/P95/P99 (line chart over time)                │
│ Avg Score per Model (bar chart)                           │
└──────────────────────────────────────────────────────────┘
```

### 10.6 Embedding Validation (`/embeddings`)

```
┌──────────────────────────────────────────────────────────┐
│ Embedding Validation                                     │
├──────────────────────────────────────────────────────────┤
│ Summary Cards:                                            │
│ Avg Similarity 0.78 | Avg Relevance 0.82 | Pass Rate 85% │
├──────────────────────────────────────────────────────────┤
│ Similarity Distribution (histogram)                       │
│ Relevance vs Similarity (scatter plot)                    │
├──────────────────────────────────────────────────────────┤
│ Failed Embeddings (table, where relevance < threshold)    │
│ Run ID    | Query                  | Similarity | Rel.    │
│ uuid      | "what is X"           | 0.62       | 0.45 ❌ │
│ ...                                                       │
├──────────────────────────────────────────────────────────┤
│ Trend (line chart: avg similarity + relevance over time)  │
└──────────────────────────────────────────────────────────┘
```

### 10.7 Feedback (`/feedback`)

```
┌──────────────────────────────────────────────────────────┐
│ Human Feedback                                           │
├──────────────────────────────────────────────────────────┤
│ Summary: Total 42 | Likes 35 | Dislikes 7 | Avg 4.2/5   │
├──────────────────────────────────────────────────────────┤
│ Feedback Distribution (pie: accurate 60%, hallucination   │
│ 15%, irrelevant 10%, incomplete 10%, other 5%)           │
├──────────────────────────────────────────────────────────┤
│ Feedback List (table):                                    │
│ Date     | Source       | Rating | Category    | Comment │
│ 14:30    | url          | 5 ★    | accurate    | "Great" │
│ 14:28    | url          | 2 ★    | hallucinat. | "Wrong" │
│ ...                                                       │
└──────────────────────────────────────────────────────────┘
```

### 10.8 Regressions (`/regressions`)

```
┌──────────────────────────────────────────────────────────┐
│ Regressions                                     [24h ▼] │
├──────────────────────────────────────────────────────────┤
│ Comparison Periods:                                       │
│ Current: 2026-05-29 00:00 → 2026-05-29 14:30             │
│ Baseline: 2026-05-28 00:00 → 2026-05-28 23:59            │
├──────────────────────────────────────────────────────────┤
│ Signals Table:                                            │
│ Metric         | Current | Baseline | Δ      | Status    │
│ Avg Score      | 0.82    | 0.89     | -0.07  | 🟡 minor  │
│ Pass Rate      | 88%     | 94%      | -6%    | 🟡 minor  │
│ Avg Latency    | 2.3s    | 2.1s     | +200ms | ✅ ok     │
│ Error Rate     | 1%      | 1%       | 0%     | ✅ ok     │
│ Embedding Rel. | 0.78    | 0.85     | -0.07  | 🟡 minor  │
│ Neg. Feedback  | 3%      | 2%       | +1%    | ✅ ok     │
├──────────────────────────────────────────────────────────┤
│ Top Regressed Runs:                                       │
│ [run ID] source  score 0.30 (prev similar: 0.85)         │
│   → [view detail] [compare with previous]                 │
└──────────────────────────────────────────────────────────┘
```

### 10.9 Debug (`/debug`)

```
┌──────────────────────────────────────────────────────────┐
│ Debug                                                    │
├──────────────────────────────────────────────────────────┤
│ Live Pipeline Monitor (SSE feed)                         │
│ [14:30:12] pipeline_start  {run_id: "uuid"}              │
│ [14:30:13] stage_complete  {stage: "compress", ...}      │
│ [14:30:18] stage_complete  {stage: "process", ...}       │
│ [14:30:20] llm_call        {model: "qwen3-coder", ...}   │
│ [14:30:25] pipeline_complete {score: 0.85, ...}          │
├──────────────────────────────────────────────────────────┤
│ System Status:                                            │
│ ✅ Qdrant     | localhost:6333                            │
│ ✅ Ollama     | http://localhost:11434                    │
│ ✅ OpenRouter | key configured                            │
│ ✅ SQLite     | ~/.media2rag/data.db                      │
│ ✅ Workspace  | ~/.media2rag/workspace/                   │
├──────────────────────────────────────────────────────────┤
│ Judge Settings:                                           │
│ Enabled: ✅  | Model: nemotron-3-super-120b:free          │
│ Fallback: qwen3-coder:free | Frequency: every call       │
├──────────────────────────────────────────────────────────┤
│ Config Snapshot: JSON                                     │
└──────────────────────────────────────────────────────────┘
```

---

## 11. API Endpoints

### 11.1 Overview

```
GET /api/debug/overview?days=7
Response 200:
{
    "overallScore": 0.85,
    "pipelinePassRate": 92.0,
    "pipelineFailRate": 8.0,
    "documentsProcessed": 143,
    "avgLatencyMs": 2300.5,
    "totalCost": 0.42,
    "radar": {
        "quality": 0.85,
        "relevance": 0.90,
        "faithfulness": 0.80,
        "helpfulness": 0.88
    },
    "latestPipeline": {
        "id": "uuid",
        "source": "https://example.com",
        "score": 0.87,
        "stageScores": { "compress": 0.95, "process": 0.88, "assemble": 0.92, "judge": 0.85 }
    },
    "topFailures": [
        { "id": "uuid", "source": "https://bad.com", "score": 0.20 }
    ]
}
```

### 11.2 Timeline

```
GET /api/debug/timeline?days=7
Response 200:
{
    "days": 7,
    "points": [
        { "date": "2026-05-23", "avgScore": 0.82, "runs": 5 },
        { "date": "2026-05-24", "avgScore": 0.84, "runs": 8 },
        ...
    ]
}
```

### 11.3 Pipeline List

```
GET /api/debug/pipeline?limit=20&status=&source_type=&min_score=&max_score=
Response 200:
[
    {
        "id": "uuid",
        "source": "https://example.com",
        "source_type": "article",
        "score": 0.85,
        "total_tokens": 12340,
        "total_latency_ms": 2340.5,
        "total_cost": 0.0015,
        "status": "completed",
        "created_at": "2026-05-29T14:30:00Z",
        "stages": [
            { "name": "compress", "score": 0.95, "latency_ms": 2300.5, "tokens": 4500, ... },
            { "name": "process", "score": 0.88, "latency_ms": 5100.0, "tokens": 8000, ... }
        ],
        "judge": {
            "model": "nemotron-3-super-120b:free",
            "scores": { "quality": 0.85, "relevance": 0.90, "faithfulness": 0.80 },
            "reasoning": "The answer is accurate..."
        },
        "feedback": {
            "rating": 4,
            "like_dislike": "like",
            "category": "accurate"
        }
    }
]
```

### 11.4 Pipeline Detail

```
GET /api/debug/pipeline/{id}
Response 200:
{
    "id": "uuid",
    "source": "https://example.com",
    "source_type": "article",
    "status": "completed",
    "score": 0.85,
    "total_tokens": 12340,
    "total_latency_ms": 2340.5,
    "total_cost": 0.0015,
    "error": "",
    "pipeline_version": "abc1234",
    "config_snapshot": "{...}",
    "created_at": "2026-05-29T14:30:00Z",
    "completed_at": "2026-05-29T14:30:25Z",
    "stages": [ ... ],
    "llm_calls": [
        {
            "id": "call-uuid",
            "model": "deepseek/deepseek-v4-flash:free",
            "operation": "compress",
            "prompt_tokens": 4500,
            "completion_tokens": 800,
            "latency_ms": 1230.5,
            "cost": 0.0,
            "status": "ok"
        }
    ],
    "judge": {
        "id": "judge-uuid",
        "judge_model": "nemotron-3-super-120b:free",
        "judge_type": "quality",
        "score": 0.85,
        "reasoning": "The answer is accurate...",
        "latency_ms": 3700.0,
        "tokens_used": 3400,
        "cost": 0.0
    },
    "embedding_checks": [
        {
            "id": "emb-uuid",
            "query_text": "what is X",
            "chunk_text": "X is defined as...",
            "similarity_score": 0.85,
            "relevance_score": 0.90,
            "passed": true
        }
    ],
    "feedback": {
        "rating": 4,
        "like_dislike": "like",
        "category": "accurate",
        "comment": "Great answer"
    }
}
```

### 11.5 LLM Logs

```
GET /api/debug/logs?model=&operation=&status=&limit=50
Response 200: [ ...llm_calls array... ]

GET /api/debug/logs/{id}
Response 200: { ...single llm_call... }
```

### 11.6 Metrics

```
GET /api/debug/metrics?period=7d
Response 200:
{
    "scoreDistribution": {
        "0.0-0.2": 2,
        "0.2-0.4": 4,
        "0.4-0.6": 8,
        "0.6-0.8": 18,
        "0.8-1.0": 28
    },
    "modelComparison": [
        { "model": "deepseek:free", "avgScore": 0.85, "callCount": 45 },
        { "model": "qwen3-coder:free", "avgScore": 0.72, "callCount": 30 }
    ],
    "scoreByType": [
        { "type": "article", "avgScore": 0.82, "count": 20 },
        { "type": "video", "avgScore": 0.76, "count": 15 }
    ],
    "usage": {
        "todayTokens": 45200,
        "weekTokens": 312000,
        "monthTokens": 1200000,
        "todayCost": 0.0,
        "weekCost": 0.01,
        "monthCost": 0.05
    },
    "latency": {
        "p50": 1200.5,
        "p95": 4500.0,
        "p99": 8900.0
    }
}
```

### 11.7 Embeddings

```
GET /api/debug/embeddings?period=7d&limit=20
Response 200:
{
    "summary": {
        "avgSimilarity": 0.78,
        "avgRelevance": 0.82,
        "passRate": 0.85,
        "totalChecks": 50
    },
    "checks": [ ...embedding_checks array... ],
    "trend": [
        { "date": "2026-05-23", "avgSimilarity": 0.75, "avgRelevance": 0.80 },
        ...
    ]
}
```

### 11.8 Feedback

```
GET /api/debug/feedback?period=7d
Response 200:
{
    "summary": {
        "total": 42,
        "likes": 35,
        "dislikes": 7,
        "avgRating": 4.2,
        "categories": {
            "accurate": 25,
            "hallucination": 6,
            "irrelevant": 4,
            "incomplete": 4,
            "other": 3
        }
    },
    "feedback": [ ...feedback array... ]
}

POST /api/debug/feedback
Request: { "run_id", "llm_call_id?", "rating", "like_dislike", "category", "comment" }
Response 201: { "id": "uuid", "status": "created" }
```

### 11.9 Regressions

```
GET /api/debug/regressions?period=24h&baseline=48h&baseline_duration=24h
Response 200:
{
    "currentPeriod": {
        "start": "2026-05-29T00:00:00Z",
        "end": "2026-05-29T14:30:00Z",
        "runs": 25
    },
    "baselinePeriod": {
        "start": "2026-05-28T00:00:00Z",
        "end": "2026-05-28T23:59:59Z",
        "runs": 30
    },
    "signals": [
        {
            "name": "avgScore",
            "current": 0.82,
            "baseline": 0.89,
            "delta": -0.07,
            "deltaPercent": -7.9,
            "status": "minor",
            "direction": "down",
            "unit": "score"
        },
        {
            "name": "passRate",
            "current": 88.0,
            "baseline": 94.0,
            "delta": -6.0,
            "deltaPercent": -6.4,
            "status": "minor",
            "direction": "down",
            "unit": "percent"
        },
        ...
    ],
    "topRegressions": [
        {
            "runId": "uuid",
            "source": "https://bad.com",
            "score": 0.30,
            "baselineScore": 0.85
        }
    ]
}
```

### 11.10 Status

```
GET /api/debug/status
Response 200:
{
    "qdrant":    { "connected": true,  "details": "localhost:6333" },
    "ollama":    { "connected": true,  "details": "http://localhost:11434" },
    "openrouter":{ "connected": true,  "details": "key configured" },
    "sqlite":    { "connected": true,  "details": "~/.media2rag/data.db" },
    "workspace": { "connected": true,  "details": "~/.media2rag/workspace/" }
}
```

### 11.11 Config

```
GET /api/debug/config
Response 200:
{
    "judgeEnabled": true,
    "judgeModel": "nvidia/nemotron-3-super-120b-a12b:free",
    "judgeFallback": "qwen/qwen3-coder:free",
    "judgeFrequency": "every call",
    "pipelineModel": "openrouter/free",
    "pipelineFallback": "mistralai/mistral-nemo",
    "embeddingModel": "nomic-embed-text",
    "embeddingCheckFrequency": 1
}
```

### 11.12 Reprocess

```
POST /api/debug/reprocess/{id}
Response 200: { "status": "queued", "id": "uuid" }
```

### 11.13 Live SSE

```
GET /api/debug/live
Response: text/event-stream
event: connected\ndata: {"status":"ok"}\n\n
event: pipeline_start\ndata: {"run_id":"uuid","source":"...","timestamp":"..."}\n\n
event: stage_complete\ndata: {"run_id":"uuid","stage":"compress","score":0.95,...}\n\n
event: pipeline_complete\ndata: {"run_id":"uuid","score":0.85,...}\n\n
event: heartbeat\ndata: {"ts":1712345678}\n\n
```

---

## 12. Metrics Caching Strategy

### 12.1 Cache Keys

| Key | TTL | Computed From |
|-----|-----|--------------|
| `overview:{days}` | 60s | pipeline_runs, judge_evaluations |
| `timeline:{days}` | 120s | pipeline_runs (group by date) |
| `metrics:{period}` | 120s | pipeline_runs, llm_calls |
| `embeddings:{period}` | 300s | embedding_checks |
| `feedback:{period}` | 60s | feedback |
| `regressions:{period}:{baseline}` | 300s | pipeline_runs |

### 12.2 Invalidation

Cache is invalidated on:
- New pipeline run completes
- New feedback submitted
- Manual refresh (user action)
- Every N seconds via SSE heartbeat

---

## 13. Implementation Plan

### Phase 1: Backend Data Layer
1. Create SQLite schema (migration file)
2. Add `Tracer` struct to `internal/pipeline/` — captures stages, LLM calls, emits SSE
3. Create `internal/dashboard/` package — data access layer for all dashboard tables
4. Integrate tracer into pipeline (compress, process, assemble stages)
5. Add judge evaluation runner (decoupled, async after pipeline)
6. Add embedding check runner (sampled, async)

### Phase 2: Backend API
1. Update `internal/api/debug.go` — all new endpoints with real data
2. Metrics computation (aggregate queries)
3. Regression comparison logic
4. SSE event broadcasting (fan-out to connected clients)
5. Feedback endpoint

### Phase 3: Frontend
1. Extend types in `dashboard/src/lib/types.ts`
2. Extend API client in `dashboard/src/lib/api.ts`
3. Redesign Overview page with full KPIs, radar, timeline
4. Redesign Pipeline page with stages, judge, embeddings, feedback
5. New pages: Embeddings, Feedback, Regressions
6. Add like/dislike UI to pipeline detail
7. SSE integration for live updates

### Phase 4: Refinement
1. Metrics caching with invalidation
2. Error handling and fallbacks
3. Performance optimization (pagination, lazy loading)
4. Dark mode consistency
5. Responsive layout

---

## 14. Configuration

### config.yaml additions

```yaml
dashboard:
  enabled: true
  metrics_cache_ttl: 60        # seconds
  default_period: "7d"         # default time range

judge:
  enabled: true
  model: "nvidia/nemotron-3-super-120b-a12b:free"
  fallback_model: "qwen/qwen3-coder:free"
  frequency: "every"           # every, sampling:5 (every 5th), off
  types:
    - quality
    - relevance
    - faithfulness
    - helpfulness

embedding_checks:
  enabled: true
  sample_size: 5
  relevance_threshold: 0.7
  similarity_threshold: 0.6
  check_frequency: 1           # every N runs

pipeline:
  model: "openrouter/free"
  fallback_model: "mistralai/mistral-nemo"
  retry_count: 2
```

---

---

## 15. LLM Output Format: TypedBlock

### 15.1 Format Spec

Every LLM call in the system uses the same structured output format:

```
> type: key1=val1, key2=val2
content line 1
content line 2
<
```

**Rules:**
- Block starts with `> ` followed by the block type (required)
- Optional params after `: ` as comma-separated `key=value` pairs
- Content follows on subsequent lines (can be multiline)
- Block ends with `<` on its own line
- Multiple blocks can appear in one response
- If no `>` marker is found, the entire output is treated as a `text` block

### 15.2 TypedBlock struct (internal/model/llm.go)

```go
type TypedBlock struct {
    Type    string            `json:"type"`
    Params  map[string]string `json:"params,omitempty"`
    Content string            `json:"content"`
}
```

### 15.3 Parser (internal/llm/parse.go)

`ParseOutput(text string) ([]model.TypedBlock, error)` — used by all LLM clients (OpenRouter, Ollama) to parse structured output from models.

### 15.4 Examples from Production

| Input | Blocks |
|-------|--------|
| `> memory\nПользователя зовут Никита\n<` | `[{Type: "memory", Content: "Пользователя зовут Никита"}]` |
| `> topic: chunk=3, lang=ru\nHNSW, векторный поиск\n<` | `[{Type: "topic", Params: {chunk: "3", lang: "ru"}, Content: "HNSW, векторный поиск"}]` |
| `> topic\ntopic1\n<\n> summary\nsummary text\n<` | `[{Type: "topic", Content: "topic1"}, {Type: "summary", Content: "summary text"}]` |

### 15.5 Dashboard Integration

Pipeline stages store prompt and response in `pipeline_stages`. The response is **always** in this structured format. The dashboard displays:
- **Raw response** — full text as received from LLM
- **Parsed blocks** — extracted into a structured table showing Type → Params → Content
- **Judge evaluation** — the judge's response is also in this format with `score` and `reasoning` blocks

Example judge output:
```
> score: type=quality
0.85
<
> reasoning
The answer is accurate and directly addresses the user's question. It provides complete information without hallucination.
<
```

The dashboard parses this with `ParseOutput()` and displays score + reasoning separately.

---

## 16. Open Questions

1. Should judge evaluation be synchronous (blocks pipeline) or async (runs after)?
   — **Decision: async**. Pipeline returns immediately, judge runs in background.
2. How many embedding checks per run?
   — **Decision: 5 chunks sampled** from the retrieved set.
3. Should we store full prompt/response in SQLite (bloat) or only in files?
   — **Decision: store in SQLite** for fast access, but truncate >10K chars.
4. Cache strategy: recompute on write or on read with TTL?
   — **Decision: read with TTL** (60-300s), invalidate on SSE event.
