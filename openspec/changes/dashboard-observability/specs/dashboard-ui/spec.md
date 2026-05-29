## ADDED Requirements

### Requirement: Dashboard navigation
The dashboard SHALL provide a sidebar navigation with links to all 9 pages: Overview, Pipeline, Logs, Metrics, Embeddings, Feedback, Regressions, Debug.

#### Scenario: Sidebar navigation
- **WHEN** the dashboard loads
- **THEN** a collapsible sidebar is displayed with all page links
- **WHEN** a sidebar link is clicked
- **THEN** the page changes via SPA routing without full reload

### Requirement: Overview page
The dashboard SHALL display an Overview page with KPI cards, score radar, timeline chart, top failures, and latest pipeline run.

#### Scenario: Overview KPIs
- **WHEN** the Overview page loads
- **THEN** KPI cards for Score, Pass Rate, Documents Processed, Avg Latency, and Cost are displayed with trend arrows

#### Scenario: Score radar
- **WHEN** the Overview page loads
- **THEN** a horizontal bar chart shows scores for quality, relevance, faithfulness, and helpfulness

#### Scenario: Timeline chart
- **WHEN** the Overview page loads
- **THEN** a line chart shows average score per day for the selected period

#### Scenario: Top failures
- **WHEN** runs with score < 0.5 exist
- **THEN** they are listed in a "Top Failures" section with clickable links to pipeline detail

### Requirement: Pipeline page
The dashboard SHALL display a Pipeline page with a list of pipeline runs, expandable for detail including stages, judge, embeddings, and feedback.

#### Scenario: Pipeline list
- **WHEN** the Pipeline page loads
- **THEN** a table of pipeline runs is shown with Run ID, Source, Score, Tokens, Latency, and expand button

#### Scenario: Pipeline detail expansion
- **WHEN** a pipeline row is expanded
- **THEN** stages with scores, judge evaluation, embedding checks, and feedback are shown

#### Scenario: Stage prompt/response view
- **WHEN** a "view prompt → response" button is clicked on a stage
- **THEN** the full prompt and response are displayed in scrollable pre blocks

### Requirement: Pipeline detail page
The dashboard SHALL provide a dedicated detail page at `/pipeline/{id}` with full single-run view.

#### Scenario: Full run detail
- **WHEN** `/pipeline/{id}` is loaded
- **THEN** all run metadata, stage timeline, LLM calls table, judge evaluation, embedding checks, and feedback are displayed

#### Scenario: Stage timeline visualization
- **WHEN** stages are displayed on the detail page
- **THEN** each stage is shown as a horizontal bar with score, latency, and token count

### Requirement: LLM Logs page
The dashboard SHALL display an LLM Logs page with a filterable table of all LLM calls.

#### Scenario: Log filters
- **WHEN** the Logs page loads
- **THEN** filter dropdowns for Model, Operation, Status, and Time range are displayed

#### Scenario: Log expansion
- **WHEN** a log row is clicked
- **THEN** the full prompt, response, latency, token counts, and cost are displayed

### Requirement: Metrics page
The dashboard SHALL display a Metrics page with score distribution, model comparison, score by type, token usage, and latency percentiles.

#### Scenario: Score distribution histogram
- **WHEN** the Metrics page loads
- **THEN** a histogram of score buckets (0.0-0.2, 0.2-0.4, etc.) is shown

#### Scenario: Model comparison
- **WHEN** models with score data exist
- **THEN** a bar chart comparing average scores per model is shown

### Requirement: Embeddings page
The dashboard SHALL display an Embeddings page with similarity distribution, relevance vs similarity scatter plot, failed embeddings table, and trend chart.

#### Scenario: Embedding summary
- **WHEN** the Embeddings page loads
- **THEN** summary cards for Avg Similarity, Avg Relevance, and Pass Rate are shown

#### Scenario: Failed embeddings
- **WHEN** chunks with relevance < threshold exist
- **THEN** they are shown in a "Failed Embeddings" table

### Requirement: Feedback page
The dashboard SHALL display a Feedback page with summary stats, category distribution, and feedback list.

#### Scenario: Feedback summary
- **WHEN** the Feedback page loads
- **THEN** total feedback count, likes, dislikes, average rating, and category distribution are displayed

### Requirement: Regressions page
The dashboard SHALL display a Regressions page with period comparison, signal table with alert status, and top regressed runs.

#### Scenario: Signal comparison
- **WHEN** the Regressions page loads
- **THEN** all 7 signals are displayed with current vs baseline values and status indicators

#### Scenario: Top regressed runs
- **WHEN** regressed runs exist
- **THEN** they are listed with score and baseline comparison

### Requirement: Debug page
The dashboard SHALL display a Debug page with live pipeline SSE monitor, system status, judge settings, and config snapshot.

#### Scenario: Live monitor
- **WHEN** the Debug page loads
- **THEN** it connects to the SSE stream and displays incoming events in a scrollable log

#### Scenario: System status
- **WHEN** the Debug page loads
- **THEN** status of Qdrant, Ollama, OpenRouter, SQLite, and Workspace components is displayed with connected/disconnected indicators

### Requirement: Like/dislike in pipeline detail
The dashboard SHALL display like/dislike buttons on pipeline detail pages.

#### Scenario: Submit like
- **WHEN** a user clicks the thumbs-up button on a pipeline detail page
- **THEN** a `POST /api/debug/feedback` call is made with `like_dislike: "like"` for that run_id

#### Scenario: Submit dislike
- **WHEN** a user clicks the thumbs-down button
- **THEN** a `POST /api/debug/feedback` call is made with `like_dislike: "dislike"`
