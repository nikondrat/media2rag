## ADDED Requirements

### Requirement: Drag-and-drop queue reordering
The system SHALL allow users to reorder queue items by dragging and dropping them to a new position.

#### Scenario: Reordering items
- **WHEN** user drags item from position 3 to position 1
- **THEN** the item moves to position 1 and other items shift down

#### Scenario: Drag visual feedback
- **WHEN** user starts dragging an item
- **THEN** the item shows a drag preview and drop targets highlight

### Requirement: Reorder in QueueManager
The `QueueManager` SHALL provide a `reorder(from:to:)` method that updates the items array order.

#### Scenario: Reorder method
- **WHEN** `reorder(from: 3, to: 1)` is called
- **THEN** items array is updated with the item moved to the new index
