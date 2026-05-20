## ADDED Requirements

### Requirement: Context menu on queue items
The system SHALL display a context menu when the user right-clicks on a queue item row.

#### Scenario: Context menu for queued item
- **WHEN** user right-clicks on a queued item
- **THEN** menu shows: "Process this", "Remove", "Copy path"

#### Scenario: Context menu for completed item
- **WHEN** user right-clicks on a completed item
- **THEN** menu shows: "Open in Finder", "Open workspace", "Copy path", "Remove"

#### Scenario: Context menu for failed item
- **WHEN** user right-clicks on a failed item
- **THEN** menu shows: "Retry", "Remove", "Copy error"

### Requirement: Context menu actions
Each context menu action SHALL perform the corresponding operation on the selected queue item.

#### Scenario: Process this file
- **WHEN** user selects "Process this" from context menu
- **THEN** only that file is processed (not the entire queue)

#### Scenario: Open workspace
- **WHEN** user selects "Open workspace" from context menu on a completed item
- **THEN** Finder opens the workspace directory for that file
