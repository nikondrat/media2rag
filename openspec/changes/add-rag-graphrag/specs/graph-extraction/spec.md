## ADDED Requirements

### Requirement: Entity extraction from chunks
The system SHALL extract entities from each chunk using LLM, identifying 12 entity types: Problem, Solution, Opportunity, Skill, Resource, Market, Audience, Business, Event, Claim, Metric, Concept.

#### Scenario: Extract entities from business chunk
- **WHEN** chunk contains "Отсутствие CRM ведёт к потере 30% лидов"
- **THEN** entities extracted: Problem("Отсутствие CRM"), Metric("потеря 30% лидов")

#### Scenario: Extract entities from IT chunk
- **WHEN** chunk contains "Repository pattern solves tight coupling"
- **THEN** entities extracted: Solution("Repository pattern"), Problem("tight coupling")

### Requirement: Relation extraction from chunks
The system SHALL extract relations between entities using LLM, identifying 14 relation types: causes, enables, prevents, requires, solves, blocks, competes_with, serves, leverages, leads_to, correlates, supports, contradicts, part_of.

#### Scenario: Extract causal relation
- **WHEN** chunk contains "Плохой UX вызывает churn 40%"
- **THEN** relation extracted: causes(Problem("Плохой UX"), Metric("churn 40%"))

#### Scenario: Extract relation with provenance
- **WHEN** relation is extracted
- **THEN** source_chunk field contains reference to original chunk ID

### Requirement: Entity deduplication
The system SHALL deduplicate entities using embedding similarity (threshold 0.85) + LLM resolution for ambiguous cases.

#### Scenario: Merge similar entities
- **WHEN** entities "склады" and "warehouse" have similarity > 0.85
- **THEN** they are merged into single node with aliases

#### Scenario: LLM resolves ambiguous case
- **WHEN** similarity is 0.7-0.85 (ambiguous)
- **THEN** LLM is called to determine if entities are the same

### Requirement: Batch extraction with concurrency
The system SHALL process chunks in parallel using configurable concurrency (default: 5 concurrent LLM calls).

#### Scenario: Process 100 chunks
- **WHEN** `media2rag index` is called with 100 chunks
- **THEN** chunks are processed in parallel, progress bar shows ETA
