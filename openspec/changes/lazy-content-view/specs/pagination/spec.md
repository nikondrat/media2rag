## ADDED Requirements

### Requirement: Page-based navigation
The system SHALL split section rendering into pages when the total number of sections exceeds a threshold (200 sections). Each page SHALL contain 50 sections.

#### Scenario: Pagination for large document
- **WHEN** a document has 350 sections
- **THEN** content is split into 7 pages (50 sections each, last page has 50)

#### Scenario: No pagination for small documents
- **WHEN** a document has 45 sections
- **THEN** all sections are displayed on a single page without pagination controls

### Requirement: Page navigation controls
The system SHALL display page navigation controls when pagination is active. Controls SHALL include previous/next buttons and page number indicators.

#### Scenario: Page navigation UI
- **WHEN** pagination is active with 7 pages
- **THEN** the UI shows `← 1 2 3 ... 7 →` with the current page highlighted

#### Scenario: Page change
- **WHEN** user clicks page number 3
- **THEN** sections 100-149 are rendered and the scroll position resets to top
