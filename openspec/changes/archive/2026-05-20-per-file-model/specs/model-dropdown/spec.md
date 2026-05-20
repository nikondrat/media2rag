## ADDED Requirements

### Requirement: Model selection dropdown in QueueItemRow
The QueueItemRow view SHALL display a dropdown menu allowing the user to select the LLM backend and model for that specific file.

#### Scenario: Dropdown shows current model
- **WHEN** a file is in the queue with `backend="ollama"` and `model="gemma4:26b"`
- **THEN** the dropdown displays "gemma4:26b" as the selected value

#### Scenario: Changing model via dropdown
- **WHEN** user selects a different model from the dropdown
- **THEN** the file's `model` field is updated and the dropdown reflects the new selection

#### Scenario: Backend change updates model list
- **WHEN** user selects a different backend (Ollama ↔ OpenRouter) in the dropdown
- **THEN** the model list is refreshed for the new backend and the first available model is selected

### Requirement: Dropdown model list source
The dropdown SHALL populate its model list from `ModelManager` — Ollama models for Ollama backend, OpenRouter models for OpenRouter backend.

#### Scenario: Ollama model list
- **WHEN** backend is set to "ollama"
- **THEN** the dropdown shows models from `modelManager.ollamaModels`

#### Scenario: OpenRouter model list
- **WHEN** backend is set to "openrouter"
- **THEN** the dropdown shows models from `modelManager.openRouterModels` with display names

### Requirement: Compact dropdown design
The dropdown SHALL be compact and fit within the QueueItemRow without truncating the file name.

#### Scenario: Compact layout
- **WHEN** the dropdown is displayed in a queue item row
- **THEN** it uses a menu-style picker (not a full Picker control) to save space
