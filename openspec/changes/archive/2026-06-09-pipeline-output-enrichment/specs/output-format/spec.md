## ADDED Requirements

### Requirement: Output format structure
The system SHALL produce a Markdown file with YAML frontmatter followed by cleaned content.

#### Scenario: Frontmatter delimiter
- **WHEN** document is generated
- **THEN** output starts with `---\n` and frontmatter ends with `\n---\n\n`

#### Scenario: Content after frontmatter
- **WHEN** frontmatter is written
- **THEN** cleaned markdown content follows after the closing `---`

### Requirement: Frontmatter fields
The YAML frontmatter SHALL contain: title, source, type, author, language, domains, core_thesis, mental_models, claims, takeaways, key_terms, summary, word_count, topics.

#### Scenario: Title field
- **WHEN** document has a title
- **THEN** frontmatter contains `title: <title>` (quoted if contains special chars)

#### Scenario: Source and type
- **WHEN** document has source info from ExtractedContent
- **THEN** frontmatter contains `source: <path|url>` and `type: <doc_type>`

#### Scenario: Author and language
- **WHEN** extractor provides author and language
- **THEN** frontmatter contains `author: <name>` and `language: <code>`

#### Scenario: Domains list
- **WHEN** document has identified domains
- **THEN** frontmatter contains `domains:` with a YAML list

#### Scenario: Core thesis
- **WHEN** core thesis is extracted
- **THEN** frontmatter contains `core_thesis: <text>`

#### Scenario: Mental models
- **WHEN** mental models are extracted
- **THEN** frontmatter contains `mental_models:` with a YAML list

#### Scenario: Claims
- **WHEN** claims are extracted
- **THEN** frontmatter contains `claims:` with a YAML list of objects (text, type, confidence)

#### Scenario: Takeaways
- **WHEN** takeaways are extracted
- **THEN** frontmatter contains `takeaways:` with a YAML list

#### Scenario: Key terms
- **WHEN** key terms are extracted
- **THEN** frontmatter contains `key_terms:` with a YAML list of objects (term, definition)

#### Scenario: Summary
- **WHEN** summary is generated
- **THEN** frontmatter contains `summary: <text>` (literal block if multiline)

#### Scenario: Word count
- **WHEN** document is generated
- **THEN** frontmatter contains `word_count: <int>`

#### Scenario: Topics
- **WHEN** topics are extracted
- **THEN** frontmatter contains `topics:` with a YAML list sorted by frequency
