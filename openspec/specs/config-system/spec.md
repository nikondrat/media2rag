## ADDED Requirements

### Requirement: Config loading from multiple sources
The system SHALL load configuration from three sources with priority order: CLI flags (highest) > environment variables > `~/.media2rag/config.yaml` (lowest).

#### Scenario: Config loaded from YAML only
- **WHEN** no env vars or CLI flags are set
- **THEN** configuration is loaded from `~/.media2rag/config.yaml`

#### Scenario: Env var overrides YAML
- **WHEN** `MEDIA2RAG_LLM_DEFAULT_BACKEND=openrouter` is set and YAML has `ollama`
- **THEN** the effective config uses `openrouter`

#### Scenario: CLI flag overrides all
- **WHEN** `--llm-backend openrouter` is passed, env says `ollama`, YAML says `ollama`
- **THEN** the effective config uses `openrouter`

### Requirement: Config validation
The system SHALL validate required fields and return structured errors for missing or invalid values.

#### Scenario: Missing required field
- **WHEN** `LLM.DefaultBackend` is empty or not "ollama"/"openrouter"
- **THEN** config loading returns `ErrConfigInvalid`

#### Scenario: Valid config passes
- **WHEN** all fields have valid values
- **THEN** config loads without error

### Requirement: Config struct matches architecture
The Config struct SHALL contain sections for LLM, Pipeline, Workspace, and Server as defined in `docs/architecture.md`.

#### Scenario: Config struct access
- **WHEN** code accesses `cfg.LLM.OllamaURL`
- **THEN** it returns the configured URL or default `http://localhost:11434`
