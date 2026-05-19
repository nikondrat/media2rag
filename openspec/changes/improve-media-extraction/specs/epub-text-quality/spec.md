## ADDED Requirements

### Requirement: Reconstruct paragraphs split by inline formatting
The system SHALL merge text fragments that were artificially split by inline HTML tags (`<em>`, `<i>`, `<strong>`, `<b>`, `<span>`) into single coherent paragraphs.

#### Scenario: Book title split by italic tags
- **WHEN** an EPUB contains `In <em>The Theory of Investment Value</em>, Williams presented`
- **THEN** the extracted text is `In The Theory of Investment Value, Williams presented` as a single paragraph

#### Scenario: Nested inline tags
- **WHEN** an EPUB contains nested inline tags like `<span><em>text</em></span>`
- **THEN** the extracted text is `text` without extra line breaks

### Requirement: Detect and convert ALL-CAPS section headers
The system SHALL detect ALL-CAPS phrases that function as section headers and convert them to markdown `##` headings.

#### Scenario: ALL-CAPS header on its own line
- **WHEN** a line contains only uppercase letters, spaces, and hyphens with 3+ words and 15+ characters
- **THEN** it is converted to a `##` markdown heading

#### Scenario: ALL-CAPS header at paragraph start
- **WHEN** a paragraph begins with an ALL-CAPS phrase (3+ words, 15+ chars) followed by normal text
- **THEN** the ALL-CAPS phrase is extracted as a `##` heading and the remaining text becomes a new paragraph

#### Scenario: Normal ALL-CAPS text not converted
- **WHEN** a line contains ALL-CAPS text with fewer than 3 words or fewer than 15 characters
- **THEN** it is NOT converted to a heading

### Requirement: Clean punctuation spacing artifacts
The system SHALL remove extra spaces before punctuation marks (`,`, `.`, `;`, `:`) that result from HTML tag unwrapping.

#### Scenario: Space before comma
- **WHEN** extracted text contains `Value , Williams presented`
- **THEN** it is cleaned to `Value, Williams presented`

#### Scenario: Space before period
- **WHEN** extracted text contains `Analysis , a whole generation`
- **THEN** it is cleaned to `Analysis, a whole generation`
