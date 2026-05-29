## ADDED Requirements

### Requirement: Pipeline run capture
The system SHALL capture every pipeline execution as a `pipeline_runs` record with id, source, source_type, status, score, total_tokens, total_latency_ms, total_cost, error, pipeline_version, and timestamps.

#### Scenario: Pipeline starts
- **WHEN** a pipeline execution begins
- **THEN** a `pipeline_runs` record is created with `status: "running"` and `created_at` timestamp

#### Scenario: Pipeline completes successfully
- **WHEN** all stages complete without error
- **THEN** the record is updated with `status: "completed"`, final `score`, `total_tokens`, `total_latency_ms`, `total_cost`, and `completed_at`

#### Scenario: Pipeline fails
- **WHEN** any stage returns an error
- **THEN** the record is updated with `status: "failed"` and `error` containing the error message

### Requirement: Stage-level tracing
The system SHALL capture every pipeline stage as a `pipeline_stages` record with run_id, name, seq, status, score, latency_ms, tokens_in, tokens_out, prompt, response, model, and error.

#### Scenario: Stage completes
- **WHEN** a pipeline stage (compress, process, assemble) finishes
- **THEN** a `pipeline_stages` record is created with the stage's prompt, response, token counts, latency, score, and model used

#### Scenario: Stage prompt and response stored
- **WHEN** a stage completes with prompt and response
- **THEN** `prompt` and `response` fields contain the full text, truncated at 10K characters if longer

### Requirement: LLM call capture
The system SHALL capture every LLM API call as an `llm_calls` record with run_id, model, operation, prompt_tokens, completion_tokens, latency_ms, cost, prompt, response, status, and error_message.

#### Scenario: LLM call recorded
- **WHEN** any LLM call is made (chat, embed, judge)
- **THEN** an `llm_calls` record is created with full call metadata

#### Scenario: LLM call cost calculated
- **WHEN** an LLM call completes with token counts
- **THEN** `cost` is calculated as `(prompt_tokens / 1_000_000 * input_price) + (completion_tokens / 1_000_000 * output_price)` using pricing from the models.dev API

### Requirement: Trace hook in pipeline
The system SHALL wrap every stage execution with a tracing function that records start time, end time, prompt, response, tokens, and score.

#### Scenario: Stage wrapped with tracing
- **WHEN** a stage function is called
- **THEN** the tracer captures the stage metadata before and after execution
- **AND** the tracer writes records to SQLite after stage completion

### Requirement: Prompt version tracking
The system SHALL track prompt versions for each pipeline stage in a `prompt_versions` table with name, version, prompt_text, model, parameters, and created_at.

#### Scenario: Prompt version recorded
- **WHEN** a pipeline stage executes with a prompt template
- **THEN** the prompt text and model are recorded with an incremented version number
- **WHEN** the same prompt template is used again without changes
- **THEN** the existing version is reused without creating a new entry

### Requirement: Configuration snapshot
The system SHALL capture the active configuration as a JSON blob at the time each pipeline run starts, stored in `pipeline_runs.config_snapshot`.

#### Scenario: Config captured at pipeline start
- **WHEN** a pipeline run begins
- **THEN** current config (judge settings, model selection, thresholds) is serialized into `config_snapshot`

### Requirement: SSE events on pipeline progress
The system SHALL emit SSE events for pipeline start, stage completion, and pipeline completion.

#### Scenario: Pipeline SSE events emitted
- **WHEN** a pipeline starts
- **THEN** an SSE `pipeline_start` event is broadcast with run_id and source
- **WHEN** a stage completes
- **THEN** an SSE `stage_complete` event is broadcast with run_id, stage name, score, latency
- **WHEN** a pipeline completes
- **THEN** an SSE `pipeline_complete` event is broadcast with run_id and score
