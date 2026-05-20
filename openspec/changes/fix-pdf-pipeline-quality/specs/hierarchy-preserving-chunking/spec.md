## ADDED Requirements

### Requirement: SemanticChunker supports hierarchy-preserving mode
The `SemanticChunker` SHALL accept a `preserve_sections: bool` parameter. When True, chunk boundaries are restricted to H1/H2 heading boundaries only, never splitting within a section.

#### Scenario: Default mode splits at any heading level
- **WHEN** `preserve_sections=False` (default)
- **THEN** chunks can split at H1, H2, H3, H4, H5, H6 boundaries

#### Scenario: Hierarchy mode splits only at H1/H2
- **WHEN** `preserve_sections=True`
- **THEN** chunks split only at H1 (#) and H2 (##) boundaries, never at H3+

### Requirement: Sections exceeding target size become oversized chunks
When a single section (content between two H2 headings) exceeds `TARGET_SIZE`, the chunker SHALL create a chunk larger than `TARGET_SIZE` rather than splitting the section.

#### Scenario: Large Phase section stays intact
- **WHEN** "Фаза А. Остановка предыдущего тренда" section is 12,000 chars (TARGET_SIZE=8,000)
- **THEN** it becomes a single 12,000-char chunk, not split

#### Scenario: Small sections are grouped
- **WHEN** three consecutive H2 sections total 6,000 chars
- **THEN** they are grouped into a single chunk

### Requirement: Related sections are detected and grouped
The chunker SHALL detect related sections by heading pattern (common prefix + numbering like "Фаза А", "Фаза Б", "Phase A", "Phase B", "Глава 1", "Глава 2") and keep them together when they fit within `TARGET_SIZE`.

#### Scenario: Wyckoff phases are grouped
- **WHEN** headings follow pattern "Фаза [letter]" or "Phase [letter]"
- **THEN** consecutive phases are grouped into the same chunk if total size permits

#### Scenario: Unrelated sections are not grouped
- **WHEN** headings have no common pattern (e.g., "Введение", "Заключение", "Приложение")
- **THEN** each section is treated independently for grouping

### Requirement: Overlap is applied only between sections
When `preserve_sections=True`, the `OVERLAP` content SHALL be added only at section boundaries, not within a section.

#### Scenario: Overlap includes end of previous section
- **WHEN** chunk boundary falls between Section A and Section B
- **THEN** the last 800 chars of Section A are included at the start of Section B's chunk

#### Scenario: No overlap within a section
- **WHEN** a section is split into multiple chunks (because it exceeds TARGET_SIZE even in hierarchy mode)
- **THEN** overlap is applied between the sub-chunks of that section

### Requirement: Chunk metadata includes section info
Each `Chunk` SHALL include `section_name` and `section_level` fields when `preserve_sections=True`, enabling downstream processors to understand context.

#### Scenario: Chunk has section metadata
- **WHEN** a chunk is created from "## Фаза А. Остановка предыдущего тренда"
- **THEN** `chunk.section_name` = "Фаза А. Остановка предыдущего тренда" and `chunk.section_level` = 2

### Requirement: Fallback to paragraph splitting when no headings exist
When text contains no H1/H2 headings, the chunker SHALL fall back to paragraph-based splitting (existing behavior) regardless of `preserve_sections` setting.

#### Scenario: Plain text uses paragraph boundaries
- **WHEN** text has no markdown headings
- **THEN** chunks are split at paragraph boundaries (\n\n) as before
