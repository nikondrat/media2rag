## MODIFIED Requirements

### Requirement: LLM client fallback
The system SHALL support a multi-level fallback chain: free OpenRouter model → alternative free model → cheap paid model → error.

#### Scenario: Three-level fallback
- **WHEN** the primary model returns error (rate limit, timeout, 5xx)
- **THEN** the request is retried with the first fallback model
- **WHEN** the first fallback also returns error
- **THEN** the request is retried with the second fallback
- **WHEN** all fallbacks fail
- **THEN** an error is returned with details of all failures

#### Scenario: Per-role fallback configuration
- **WHEN** a pipeline operation calls an LLM
- **THEN** the pipeline model chain is used (openrouter/free → deepseek/deepseek-v4-flash:free → mistralai/mistral-nemo)
- **WHEN** a judge operation calls an LLM
- **THEN** the judge model chain is used (nvidia/nemotron-3-super-120b-a12b:free → qwen/qwen3-coder:free → qwen/qwen3.5-9b)

### Requirement: Cost tracking
The system SHALL track the cost of every LLM call based on token usage and model pricing.

#### Scenario: Cost computed from tokens
- **WHEN** an LLM call completes with `prompt_tokens: 500` and `completion_tokens: 100`
- **THEN** `cost = (500 / 1_000_000 * input_price) + (100 / 1_000_000 * output_price)` is stored in the `llm_calls` record

#### Scenario: Free model cost
- **WHEN** a free model is used (e.g., `deepseek-v4-flash:free`)
- **THEN** the cost is 0.0

### Requirement: Structured output format (TypedBlock)
All LLM clients SHALL use the `TypedBlock` format for structured responses.

#### Scenario: Judge response in TypedBlock format
- **WHEN** a judge LLM returns `"> score: type=quality\n0.85\n<\n> reasoning\nThe answer is correct\n<"`
- **THEN** `ParseOutput()` parses it into two blocks: `{Type:"score", Params:{type:"quality"}, Content:"0.85"}` and `{Type:"reasoning", Content:"The answer is correct"}`

#### Scenario: Score block parsed for numeric value
- **WHEN** a score block exists with Content "8/10"
- **THEN** the numeric score 0.8 is extracted
- **WHEN** Content is "0.85"
- **THEN** the numeric score 0.85 is extracted

### Requirement: Model pricing data
The system SHALL use pricing data from the models.dev API to calculate cost per LLM call.

#### Scenario: Pricing fetched from models.dev
- **WHEN** the application starts or pricing is first needed
- **THEN** pricing data is fetched from `https://models.dev/api.json` and cached locally in config
- **WHEN** a model's pricing is not found in the cached data
- **THEN** cost defaults to 0.0 and a warning is logged
