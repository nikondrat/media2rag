## MODIFIED Requirements

### Requirement: Content loading in DetailView
The DetailView SHALL load Markdown content using memory-mapped I/O instead of `String(contentsOf:)`. Content SHALL be rendered lazily using `LazyVStack` with section-level granularity.

#### Scenario: Opening a completed document
- **WHEN** user selects a completed item in the sidebar
- **THEN** the file is memory-mapped, section index is built, and visible sections are rendered

#### Scenario: Switching preview modes
- **WHEN** user switches between "Предпросмотр", "Промежуточный", "Исходник" modes
- **THEN** formatted mode uses lazy section rendering, raw modes load full file as before

## ADDED Requirements

### Requirement: LazyVStack section rendering
The system SHALL use `LazyVStack` to render Markdown sections, loading content only for visible sections.

#### Scenario: Scrolling through document
- **WHEN** user scrolls through a 500-section document
- **THEN** only ~10-20 visible sections are rendered at any time, others are lazy-loaded

### Requirement: Formatted preview with lazy sections
The formatted preview SHALL parse each section's Markdown on-demand and render it with appropriate styling (headings, lists, blockquotes).

#### Scenario: Rendering a heading section
- **WHEN** a section with `## Thesis` heading is rendered
- **THEN** it displays as `.title3` font with semibold weight

#### Scenario: Rendering a bullet list section
- **WHEN** a section with bullet items is rendered
- **THEN** each item shows with a bullet point and accent color
