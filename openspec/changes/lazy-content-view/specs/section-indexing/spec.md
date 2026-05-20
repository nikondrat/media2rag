## ADDED Requirements

### Requirement: Section index building
The system SHALL scan a memory-mapped Markdown file to build an index of all sections (headings) with their offsets and lengths. The index SHALL be built in a single pass over the file data.

#### Scenario: Building index from mapped data
- **WHEN** a Markdown file is opened
- **THEN** the system scans for `#`, `##`, `###` headings and builds `SectionIndex[]` with title, offset, length, level

#### Scenario: Index handles frontmatter
- **WHEN** the file starts with YAML frontmatter (`---` ... `---`)
- **THEN** frontmatter is skipped and not included in the section index

#### Scenario: Index performance
- **WHEN** building index for a 10MB file
- **THEN** the scan completes in under 100ms

### Requirement: Section data retrieval
The system SHALL retrieve individual section content from the mapped data using the pre-built index offsets.

#### Scenario: Retrieving section content
- **WHEN** section at index 5 is requested
- **THEN** the system reads `mappedData.subdata(in: offset..<offset+length)` and decodes as UTF-8 String

#### Scenario: Section includes heading
- **WHEN** a section is retrieved
- **THEN** the content includes the heading line and all content until the next heading

### Requirement: Section content caching
The system SHALL cache retrieved section content to avoid re-reading from mapped data on re-render.

#### Scenario: Cached section re-render
- **WHEN** a previously viewed section is scrolled back into view
- **THEN** the cached content is used without re-reading from the file
