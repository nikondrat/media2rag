## Context

media2rag has a Go backend with a Svelte SPA dashboard, but the debug API returns mock data. Pipeline code has no tracing — no stage timing, no token counting, no LLM call capture. SQLite is used for workspace metadata but not for operational metrics. Qdrant stores vectors but has no quality validation. The LLM output format (`TypedBlock`) exists and is used across all LLM clients but is not documented.

Full design document: `docs/plans/2026-05-29-dashboard-design.md`

## Goals / Non-Goals

**Goals:**
- Capture every pipeline run: stages, LLM calls, prompts, responses, tokens, latency, score, cost
- Evaluate output quality via LLM-as-judge using free OpenRouter models with paid fallback
- Validate embedding quality (Qdrant similarity + LLM relevance scoring)
- Collect human feedback (like/dislike, rating, category, comment)
- Detect regressions across 7 signals (score, pass rate, latency, tokens, errors, embeddings, feedback)
- Stream live pipeline events via SSE to the dashboard
- Build 9-page Svelte SPA with drill-down navigation and live updates
- Use exact `TypedBlock` format (`> type: params\ncontent\n<`) for all LLM-structured outputs

**Non-Goals:**
- Real-time alerting (email/Slack/PagerDuty) — dashboard-only for now
- A/B experiment framework — regression detection only
- Prompt versioning UI — prompt versions are tracked in DB but no management UI
- Multi-user auth — dashboard runs locally, single user
- Horizontal scaling — single-process Go server

## Decisions

1. **SQLite over dedicated timeseries DB** — The volume is low (hundreds of runs/day, not millions). SQLite is zero-ops, embedded, and sufficient for local observability. If scale becomes an issue, we add retention policies and metrics_cache TTL.

2. **Async judge evaluation** — Judge runs after pipeline returns, not blocking the response. Pipeline returns immediately with partial data; SSE pushes judge results when ready. Trade-off: dashboard shows "pending" state briefly.

3. **LLM fallback chain** — Free models first (more reliable than expected), cheap paid as fallback. The chain is configurable per role (pipeline vs judge). Cost tracking ensures we know when fallback fires.

4. **Full prompt/response in SQLite** — Stored as TEXT columns with size limit (~10K chars). Beyond that, content is truncated with a marker. This avoids filesystem I/O for the common case while keeping the DB manageable (text is cheap at this scale).

5. **Metrics cache with TTL, no invalidation hooks** — Simpler to let cache expire (60-300s) than to wire invalidation into every write path. SSE events serve as soft invalidation hints on the frontend.

6. **`TypedBlock` format for all structured output** — Every LLM call that needs structured data uses the `> type: params\ncontent\n<` format. This is already implemented in all LLM clients. The only change is documentation and ensuring judge prompts also use this format.

7. **Svelte 5 with shadcn-svelte** — Already in the project. Continuing with this stack. Use `$state()` runes for reactivity, svelte-spa-router for routing.

## Risks / Trade-offs

- **Judge model rate limits** — Free OpenRouter models may be rate-limited. Mitigation: fallback chain + retry with backoff. On persistent failure, skip judge and log.
- **SSE connection stability** — Browser may drop SSE on sleep/resume. Mitigation: auto-reconnect with 3s delay, heartbeat every 30s.
- **SQLite write contention** — Pipeline writes stages/LLM calls rapidly. Mitigation: use WAL mode, batch inserts where possible.
- **Dashboard as subpath** — Dashboard runs at `/` with SPA routing, API at `/api/`. The SPA handler must fall back to `index.html` for all non-API, non-file paths. Already implemented.
- **Cost tracking accuracy** — Free models cost $0 but fallback models have real cost. Mitigation: use models.dev pricing data cached in config.
