## 1. Backend — SQLite Schema & Data Layer

- [x] 1.1 Create `internal/dashboard/store.go` — SQLite schema initialization with all 8 tables (pipeline_runs, pipeline_stages, llm_calls, judge_evaluations, embedding_checks, feedback, metrics_cache, prompt_versions)
- [x] 1.2 Implement `internal/dashboard/tracer.go` — Tracer struct with SaveRun(), SaveStage(), SaveLLMCall() methods
- [x] 1.3 Implement `internal/dashboard/metrics.go` — aggregate query functions for overview, timeline, metrics, embeddings, feedback, regressions
- [x] 1.4 Implement metrics_cache with TTL-based read/write/invalidation logic
- [x] 1.5 Implement SSE broadcaster (`internal/dashboard/sse.go`) — fan-out to multiple clients with typed events

## 2. Backend — Pipeline Tracing

- [x] 2.1 Add tracer integration into pipeline.go — wrap each stage execution with timing and capture
- [x] 2.2 Add tracer integration into compress/process/assemble stage functions
- [x] 2.3 Ensure every LLM call through all clients creates an llm_calls record
- [x] 2.4 Add pipeline_version (git commit hash) and config_snapshot capture
- [x] 2.5 Verify TypedBlock format is used for all structured LLM outputs in pipeline

## 3. Backend — LLM-as-Judge

- [x] 3.1 Create `internal/judge/judge.go` — Judge runner with 4 types (quality, relevance, faithfulness, helpfulness)
- [x] 3.2 Implement judge prompt templates per type using TypedBlock format
- [x] 3.3 Implement score parsing from TypedBlock judge response (`> score\n0.85\n<`)
- [x] 3.4 Implement weighted score aggregation and pipeline_runs.score update
- [x] 3.5 Wire async judge execution after pipeline completion

## 4. Backend — Embedding Validation

- [x] 4.1 Create `internal/embedcheck/check.go` — Embedding check runner
- [x] 4.2 Implement Qdrant similarity query + LLM relevance check per chunk
- [x] 4.3 Store results in embedding_checks table with pass/fail determination

## 5. Backend — LLM Client Fallback & Cost

- [x] 5.1 Implement fallback chain in `internal/llm/resolver.go` — retry with next model on error
- [x] 5.2 Add cost calculation to llm_calls using models.dev pricing data
- [x] 5.3 Create per-role fallback configuration (pipeline chain vs judge chain)

## 6. Backend — Feedback & Regressions

- [x] 6.1 Implement `POST /api/debug/feedback` handler
- [x] 6.2 Implement `GET /api/debug/feedback` handler with summary statistics
- [x] 6.3 Implement `GET /api/debug/regressions` handler with 7 signals and alert thresholds
- [x] 6.4 Implement top regressed runs identification

## 7. Backend — API Endpoints

- [x] 7.1 Rewrite `GET /api/debug/overview` with real data
- [x] 7.2 Rewrite `GET /api/debug/timeline` with real data
- [x] 7.3 Rewrite `GET /api/debug/pipeline` and `GET /api/debug/pipeline/{id}` with real data
- [x] 7.4 Rewrite `GET /api/debug/logs` and `GET /api/debug/logs/{id}` with real data
- [x] 7.5 Rewrite `GET /api/debug/metrics` with real data
- [x] 7.6 Rewrite `GET /api/debug/status` with real data
- [x] 7.7 Add `GET /api/debug/embeddings` endpoint
- [x] 7.8 Add SSE endpoint with all event types broadcasting

## 8. Frontend — Types & API Client

- [x] 8.1 Extend `types.ts` with all new types (JudgeEvaluation, EmbeddingCheck, Feedback, Regression, MetricsCache, etc.)
- [x] 8.2 Extend `api.ts` with all new API functions (fetchEmbeddings, fetchFeedback, submitFeedback, fetchRegressions, etc.)
- [x] 8.3 SSE integration — connect to live stream, dispatch events to store

## 9. Frontend — Pages

- [x] 9.1 Redesign OverviewPage — KPI cards, radar, timeline chart, failures, latest pipeline
- [x] 9.2 Redesign PipelinePage — full stages with expand, judge with scores, embeddings, feedback, like/dislike
- [x] 9.3 Create PipelineDetail page — stage timeline visualization, LLM calls table, full run view
- [x] 9.4 Redesign LogsPage — enhanced filters, prompt/response display
- [x] 9.5 Redesign MetricsPage — score distribution, model comparison, latency percentiles
- [x] 9.6 Create EmbeddingsPage — similarity distribution, relevance scatter, failed embeddings table, trend
- [x] 9.7 Create FeedbackPage — summary stats, category distribution, feedback list
- [x] 9.8 Create RegressionsPage — signal table with status, top regressed runs
- [x] 9.9 Redesign DebugPage — live SSE monitor, system status, settings

## 10. Config & Documentation

- [x] 10.1 Add config.yaml sections for judge, embedding_checks, dashboard
- [ ] 10.2 Document TypedBlock output format in project docs
- [ ] 10.3 Update AGENTS.md with new commands and architecture info
