## ADDED Requirements

### Requirement: JSON progress events include workspace info
The system SHALL include workspace-related fields in JSON progress events to enable the GUI to track all artifacts for a processed file.

#### Scenario: Work directory in events
- **WHEN** any JSON event is emitted during processing
- **THEN** the event includes `work_dir` field with the workspace directory path for that file

#### Scenario: Sections list in completion event
- **WHEN** processing completes with section output
- **THEN** the `completed` event includes `sections` array with names of saved sections

#### Scenario: Images list in completion event
- **WHEN** processing completes with extracted images
- **THEN** the `completed` event includes `images` array with paths to saved image files

## ADDED Requirements

### Requirement: Workspace path in CLI args
The CLI SHALL accept `--workspace` argument to specify the workspace root directory. This replaces the previous `-o` / `--output` behavior.

#### Scenario: Workspace flag used
- **WHEN** user runs `cli.py file.pdf --workspace /my/docs`
- **THEN** processing uses `/my/docs` as workspace root

### Requirement: Backward compatible -o flag
The CLI SHALL support `-o` and `--output` as aliases for `--workspace` for backward compatibility.

#### Scenario: Legacy -o flag
- **WHEN** user runs `cli.py file.pdf -o /my/docs`
- **THEN** processing uses `/my/docs` as workspace root (same as `--workspace`)
