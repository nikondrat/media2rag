## ADDED Requirements

### Requirement: Compressor LLM cleaning
The Compressor SHALL send a cleaning prompt to LLM that removes timestamps, ads, duplicates, and OCR artifacts.

#### Scenario: Clean transcript
- **WHEN** text contains YouTube timestamps and ad messages
- **THEN** LLM returns text without timestamps and ads

#### Scenario: Preserve content
- **WHEN** text contains meaningful content mixed with noise
- **THEN** only noise is removed, content is preserved

### Requirement: Compressor chunking for large texts
When text exceeds context window, Compressor SHALL split into parts, clean each, and reassemble.

#### Scenario: Split large text
- **WHEN** text is 50K chars and context window is 32K tokens
- **THEN** text is split into 2+ parts at paragraph boundaries

#### Scenario: Reassemble cleaned parts
- **WHEN** all parts are cleaned
- **THEN** they are joined back into single cleaned text
