## ADDED Requirements

### Requirement: Toast notification system
The system SHALL display toast notifications for processing completion, errors, and other significant events. Toasts SHALL auto-dismiss after 3 seconds and support a maximum queue of 3 simultaneous notifications.

#### Scenario: File completed toast
- **WHEN** a file finishes processing successfully
- **THEN** a green toast notification appears: "✅ {filename} — готово"

#### Scenario: Error toast
- **WHEN** a file processing fails
- **THEN** a red toast notification appears: "❌ {filename} — {error message}"

#### Scenario: Toast auto-dismiss
- **WHEN** a toast is displayed
- **THEN** it automatically fades out after 3 seconds

#### Scenario: Toast queue limit
- **WHEN** 3 toasts are already displayed and a new event occurs
- **THEN** the oldest toast is replaced by the new one

### Requirement: ToastManager
The system SHALL include a `ToastManager` observable object that manages the toast queue and display state.

#### Scenario: Adding a toast
- **WHEN** `ToastManager.show(message:type:)` is called
- **THEN** the toast is added to the display queue and shown

#### Scenario: Dismissing a toast
- **WHEN** user swipes or clicks dismiss on a toast
- **THEN** the toast is removed from the queue
