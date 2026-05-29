## ADDED Requirements

### Requirement: LLM-as-judge evaluation
The system SHALL evaluate pipeline output quality using a separate LLM call after the pipeline completes.

#### Scenario: Judge runs after pipeline
- **WHEN** a pipeline run completes with status "completed"
- **THEN** the judge evaluation is triggered asynchronously for each configured judge type

#### Scenario: Judge creates evaluation record
- **WHEN** the judge LLM returns an evaluation
- **THEN** a `judge_evaluations` record is created with judge_model, judge_type, prompt, response, score, reasoning, passed, latency_ms, tokens_used, and cost

### Requirement: Four judge types
The system SHALL support four evaluation types: quality, relevance, faithfulness, and helpfulness.

#### Scenario: Quality evaluation
- **WHEN** quality judge runs
- **THEN** the judge prompt asks the LLM to evaluate correctness, completeness, and clarity on a 1-10 scale
- **AND** the response is parsed for a numeric score

#### Scenario: Relevance evaluation
- **WHEN** relevance judge runs
- **THEN** the judge prompt asks whether the answer directly addresses the user's question on a 1-10 scale

#### Scenario: Faithfulness evaluation
- **WHEN** faithfulness judge runs
- **THEN** the judge prompt asks whether the answer contains information not present in the provided context on a 1-10 scale

#### Scenario: Helpfulness evaluation
- **WHEN** helpfulness judge runs
- **THEN** the judge prompt asks how helpful the answer is on a 1-10 scale

### Requirement: Weighted score aggregation
The system SHALL compute a weighted pipeline score from judge evaluation scores.

#### Scenario: Weighted score computed
- **WHEN** all judge types have completed
- **THEN** `pipeline_runs.score` is updated as `quality * 0.4 + relevance * 0.3 + faithfulness * 0.2 + helpfulness * 0.1`

### Requirement: Judge score parsing
The system SHALL parse a numeric score from the judge LLM response.

#### Scenario: Score parsed from response
- **WHEN** the judge responds with "Score: 8/10"
- **THEN** the parsed score is 0.8
- **WHEN** the judge responds with "Rating: 7"
- **THEN** the parsed score is 0.7
- **WHEN** no score pattern is found
- **THEN** the score defaults to 0.5 and a warning is logged

### Requirement: Judge output in TypedBlock format
The judge LLM SHALL use the TypedBlock format for structured output.

#### Scenario: Judge response parsed as TypedBlock
- **WHEN** the judge responds with "> score: type=quality\n0.85\n<\n> reasoning\nThe answer is correct\n<"
- **THEN** the score block is parsed for the numeric score
- **AND** the reasoning block is stored as the judge's reasoning field
