## ADDED Requirements

### Requirement: Per-file backend and model fields
The `QueueItem` model SHALL include optional `backend: String?` and `model: String?` fields. When nil, the system SHALL use global settings values.

#### Scenario: New item gets defaults
- **WHEN** a file is added to the queue
- **THEN** `item.backend` is set to `settings.backend` and `item.model` to `settings.model`

#### Scenario: Per-file override
- **WHEN** user changes the model for a specific file
- **THEN** that file's `model` field is updated and used during processing

#### Scenario: Nil falls back to settings
- **WHEN** a file has `model = nil`
- **THEN** the processing uses `settings.model` as the model

### Requirement: CLI per-file arguments
The system SHALL pass `--backend` and `--model` arguments to CLI for each file based on the file's own settings, falling back to global settings if not set.

#### Scenario: Processing with per-file model
- **WHEN** processing a file with `backend="ollama"` and `model="gemma4:26b"`
- **THEN** CLI is called with `--backend ollama --model gemma4:26b`

#### Scenario: Processing with global defaults
- **WHEN** processing a file with `backend=nil` and `model=nil`
- **THEN** CLI is called with `--backend <settings.backend> --model <settings.model>`
