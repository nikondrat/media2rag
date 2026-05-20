## ADDED Requirements

### Requirement: Process single file action
The system SHALL allow processing a single file from the queue without processing all remaining items.

#### Scenario: Process selected file
- **WHEN** user triggers "Process this" action on a queued item
- **THEN** only that file is processed, queue order is preserved

### Requirement: Open workspace action
The system SHALL allow opening the workspace folder for a completed file directly from the UI.

#### Scenario: Open workspace from context menu
- **WHEN** user selects "Open workspace" on a completed item
- **THEN** Finder opens the workspace directory containing all artifacts

### Requirement: Copy path action
The system SHALL allow copying the output file path to the clipboard.

#### Scenario: Copy output path
- **WHEN** user selects "Copy path" on a completed item
- **THEN** the output file path is copied to clipboard and a toast confirms
