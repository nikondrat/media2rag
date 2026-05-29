## Why

media2rag currently has no observability layer. Pipeline runs execute without any tracing, LLM calls are invisible, and there is no way to measure quality, detect regressions, or collect human feedback. Without this data, it is impossible to systematically improve the RAG pipeline. A production-grade observability dashboard is needed — matching what LangSmith, Arize Phoenix, and Datadog provide for LLM applications.

## What Changes

- Add SQLite tables for pipeline tracing, LLM calls, judge evaluations, embedding validation, feedback, metrics cache, and prompt versions
- Add `Tracer` to pipeline that captures every stage (prompt, response, tokens, latency, score) and every LLM call
- Implement LLM-as-judge evaluation (4 types: quality, relevance, faithfulness, helpfulness) using free OpenRouter models with paid fallback
- Implement embedding quality validation using Qdrant similarity + LLM-judged relevance
- Add human feedback system (like/dislike, 1-5 rating, category, comment)
- Implement regression detection comparing current vs baseline periods across 7 signals
- Add SSE event stream for live dashboard updates
- Build Svelte SPA with 9 pages: Overview, Pipeline (list+detail), LLM Logs, Metrics, Embeddings, Feedback, Regressions, Debug
- Add model fallback chain: free OpenRouter → cheap paid → error
- Add typed block output format documentation (existing `TypedBlock` format)

## Capabilities

### New Capabilities

- `pipeline-tracing`: Capture every pipeline stage and LLM call with full prompt/response, tokens, latency, score, cost
- `llm-judge`: LLM-as-judge evaluation of answer quality, relevance, faithfulness, and helpfulness
- `embedding-validation`: Validate retrieved chunk quality using Qdrant similarity + LLM relevance scoring
- `human-feedback`: Like/dislike, star rating, category, and comment submission per pipeline run
- `regression-detection`: Compare current vs baseline metrics across 7 signals with alert thresholds
- `live-monitoring`: SSE event stream broadcasting pipeline events to dashboard in real time
- `metrics-cache`: Aggregate pipeline metrics with periodic recomputation and TTL-based cache
- `dashboard-ui`: Svelte SPA with 9 pages, drill-down navigation, filters, and live updates

### Modified Capabilities

- `llm-client`: Add fallback chain support (free → cheap paid → error), cost tracking, and structured output format enforcement

## Impact

- **internal/pipeline/**: All stages need tracing hooks (compress, process, assemble)
- **internal/llm/**: Client wrappers need cost calculation and fallback support
- **internal/api/debug.go**: 13 endpoints need real data instead of mock responses
- **internal/api/server.go**: Add SSE broadcaster, new route registration
- **dashboard/**: New Svelte pages, API client extensions, state management for streaming
- **config.yaml**: New sections for judge, embedding checks, dashboard settings
- **SQLite schema**: 8 new tables via migration
- **Qdrant**: Used for embedding validation queries
- **models.dev API**: Pricing data for cost calculation
