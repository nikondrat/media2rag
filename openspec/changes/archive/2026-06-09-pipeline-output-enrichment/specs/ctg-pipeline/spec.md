## MODIFIED Requirements

### Requirement: Pipeline orchestrator
The system SHALL define a `Pipeline` struct that chains stages: Compress → Split → Process → Holistic → Assemble.

#### Scenario: Pipeline execution with ExtractedContent
- **WHEN** `Pipeline.Run(ctx, ExtractedContent, emitter)` is called
- **THEN** each stage executes in order, passing ExtractedContent.Content through Compress/Split/Process

#### Scenario: Source metadata preserved
- **WHEN** Pipeline completes
- **THEN** source, type, author, language from ExtractedContent appear in final frontmatter

#### Scenario: Stage error propagation
- **WHEN** a stage returns an error
- **THEN** pipeline stops and returns the error

#### Scenario: Pipeline emits enriched events
- **WHEN** pipeline runs with holistic analysis
- **THEN** it emits `holistic_analysis` and `holistic_done` events

### Requirement: ExtractedContent input
The Pipeline SHALL accept `ExtractedContent` as input, using its Content field for text processing and metadata fields for frontmatter.

#### Scenario: Content processing
- **WHEN** Pipeline.Run receives ExtractedContent
- **THEN** ExtractedContent.Content is passed through Compress → Split → Process stages

#### Scenario: Metadata passthrough
- **WHEN** Pipeline.Run receives ExtractedContent
- **THEN** ExtractedContent.Source, DocType, Author, Language are stored for frontmatter
