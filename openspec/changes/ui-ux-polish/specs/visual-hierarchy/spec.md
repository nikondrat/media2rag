## ADDED Requirements

### Requirement: File type color coding
The system SHALL use distinct colors for different file source types in the UI (sidebar icons, badges, detail view header).

#### Scenario: PDF type color
- **WHEN** a PDF file is displayed
- **THEN** its icon and type badge use red accent color

#### Scenario: Video type color
- **WHEN** a video file is displayed
- **THEN** its icon and type badge use purple accent color

#### Scenario: Audio type color
- **WHEN** an audio file is displayed
- **THEN** its icon and type badge use green accent color

### Requirement: Backend badge color
The system SHALL color-code the LLM backend badge: orange for Ollama, blue for OpenRouter.

#### Scenario: Ollama backend badge
- **WHEN** a file uses Ollama backend
- **THEN** the backend badge is displayed in orange

#### Scenario: OpenRouter backend badge
- **WHEN** a file uses OpenRouter backend
- **THEN** the backend badge is displayed in blue

### Requirement: Per-file model indicator
The system SHALL visually indicate when a file uses a per-file model override (different from global settings).

#### Scenario: Custom model indicator
- **WHEN** a file's model differs from global settings
- **THEN** the model label shows an asterisk or italic style
