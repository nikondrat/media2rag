## MODIFIED Requirements

### Requirement: Merge chunk results
The Assembler SHALL merge ChunkResults (including claims, mental_models, key_terms, takeaways) into a single RAGDocument with deduplication and frequency sorting.

#### Scenario: Merge topics
- **WHEN** chunks have overlapping topics
- **THEN** topics are merged and deduplicated

#### Scenario: Build summary
- **WHEN** chunks have individual summaries
- **THEN** they are combined into a single document summary

#### Scenario: Merge claims
- **WHEN** chunks have claims
- **THEN** claims are merged with deduplication by text

#### Scenario: Merge mental models
- **WHEN** chunks have mental models
- **THEN** mental models are merged and deduplicated

#### Scenario: Merge key terms
- **WHEN** chunks have key terms
- **THEN** key terms are merged with deduplication by term name

#### Scenario: Merge takeaways
- **WHEN** chunks have takeaways
- **THEN** takeaways are merged and deduplicated

### Requirement: YAML frontmatter generation
The Assembler SHALL generate YAML frontmatter with all DocumentMetadata fields: title, source, type, author, language, domains, core_thesis, mental_models, claims, takeaways, key_terms, summary, word_count, topics.

#### Scenario: Full frontmatter
- **WHEN** document is generated
- **THEN** frontmatter includes all enriched fields (source, author, language, type from ExtractedContent; core_thesis, claims, mental_models, key_terms, takeaways from pipeline)

#### Scenario: Empty field handling
- **WHEN** a field has no value (empty string or empty list)
- **THEN** it is omitted from frontmatter
