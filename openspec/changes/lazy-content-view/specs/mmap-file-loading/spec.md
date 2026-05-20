## ADDED Requirements

### Requirement: Memory-mapped file reading
The system SHALL use `FileHandle.map(offset:length:)` to read Markdown files without loading the entire file into memory. The mapped data SHALL be accessed via offset ranges for individual sections.

#### Scenario: Mapping a large file
- **WHEN** a 10MB Markdown file is opened
- **THEN** `FileHandle.map()` creates a virtual memory mapping without loading file contents into RAM

#### Scenario: Reading a section from mapped data
- **WHEN** a section at offset 3072 with length 8192 is requested
- **THEN** `subdata(in: 3072..<11264)` returns only that section's bytes

#### Scenario: Fallback when mmap unavailable
- **WHEN** `FileHandle.map()` fails (unsupported filesystem)
- **THEN** the system falls back to `FileHandle.read(upToCount:)` for the requested range

### Requirement: Mapped data lifecycle
The system SHALL manage the `FileHandle` lifecycle properly: open on file access, keep alive while sections are being viewed, close when the view disappears.

#### Scenario: FileHandle cleanup
- **WHEN** the user navigates away from DetailView
- **THEN** the FileHandle is closed and mapped data is released
