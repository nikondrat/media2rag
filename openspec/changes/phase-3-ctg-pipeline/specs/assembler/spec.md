## ADDED Requirements

### Requirement: Merge chunk results
The Assembler SHALL merge ChunkResults into a single RAGDocument with deduplicated topics and combined summary.

#### Scenario: Deduplicate topics
- **WHEN** chunks have overlapping topics
- **THEN** topics are merged and deduplicated

#### Scenario: Build summary
- **WHEN** chunks have individual summaries
- **THEN** they are combined into a single document summary

### Requirement: YAML frontmatter generation
The Assembler SHALL generate YAML frontmatter with title, source, type, topics, summary, word_count.

#### Scenario: Frontmatter output
- **WHEN** document is generated
- **THEN** output starts with `---\n...metadata...\n---\n`

#### Scenario: Markdown content
- **WHEN** frontmatter is generated
- **THEN** cleaned content follows after frontmatter
