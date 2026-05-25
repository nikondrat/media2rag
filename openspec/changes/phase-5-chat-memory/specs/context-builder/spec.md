## ADDED Requirements

### Requirement: Assemble chat context
The ContextBuilder SHALL combine recent history, relevant memories, and RAG sources into chat prompt.

#### Scenario: Full context
- **WHEN** building context for a message
- **THEN** prompt includes: recent history + memories + RAG sources + question

#### Scenario: No memories
- **WHEN** no relevant memories exist
- **THEN** memories section is omitted

### Requirement: Context format
The chat context SHALL follow the defined template with sections: System, Recent History, Relevant Memories, Context, Question.

#### Scenario: Formatted prompt
- **WHEN** context is built
- **THEN** each section is separated by `--- Section ---` headers
