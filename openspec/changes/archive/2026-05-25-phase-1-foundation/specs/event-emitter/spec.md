## ADDED Requirements

### Requirement: JSON event emission
The system SHALL emit events as newline-delimited JSON to stdout when running in subprocess mode.

#### Scenario: Event emitted as JSON line
- **WHEN** an event is emitted
- **THEN** a single line of valid JSON is written to stdout

#### Scenario: Multiple events are separate lines
- **WHEN** three events are emitted sequentially
- **THEN** stdout contains three lines, each a complete JSON object

### Requirement: Event structure
Every event SHALL conform to the `Event` struct: `type`, `data` (optional), `progress` (optional), `error` (optional).

#### Scenario: Progress event
- **WHEN** `Emit(Event{Type: "progress", Progress: 0.5})` is called
- **THEN** stdout receives `{"type":"progress","progress":0.5}`

#### Scenario: Error event
- **WHEN** `Emit(Event{Type: "error", Error: "extraction failed"})` is called
- **THEN** stdout receives `{"type":"error","error":"extraction failed"}`

#### Scenario: Completed event with data
- **WHEN** `Emit(Event{Type: "completed", Data: map[string]string{"output": "file.md"}})` is called
- **THEN** stdout receives `{"type":"completed","data":{"output":"file.md"}}`

### Requirement: EventEmitter interface
The `EventEmitter` interface SHALL define `Emit(Event)` and `Done()` methods.

#### Scenario: StdoutEmitter implements interface
- **WHEN** `StdoutEmitter` is used where `EventEmitter` is expected
- **THEN** it compiles and functions correctly

#### Scenario: Done flushes output
- **WHEN** `Done()` is called
- **THEN** all buffered events are flushed and the emitter is closed
