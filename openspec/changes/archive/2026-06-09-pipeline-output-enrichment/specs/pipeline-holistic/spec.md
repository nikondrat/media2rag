## ADDED Requirements

### Requirement: Holistic document analysis
The Pipeline SHALL perform a holistic LLM analysis pass on the full cleaned text after per-chunk processing completes.

#### Scenario: Holistic pass execution
- **WHEN** all chunks are processed
- **THEN** a single LLM call is made on the entire cleaned text

#### Scenario: Core thesis extraction
- **WHEN** holistic pass runs
- **THEN** it extracts core_thesis from the full document context

#### Scenario: Domain identification
- **WHEN** holistic pass runs
- **THEN** it identifies domains (e.g., business, technology, law) from the full document

#### Scenario: Holistic override
- **WHEN** holistic result conflicts with per-chunk merge result
- **THEN** holistic result takes priority for core_thesis and domains

#### Scenario: Configurable holistic analysis
- **WHEN** PipelineConfig has HolisticAnalysis=false
- **THEN** holistic pass is skipped

### Requirement: Event emission
The holistic pass SHALL emit events for progress tracking.

#### Scenario: Holistic events
- **WHEN** holistic analysis starts
- **THEN** event `holistic_analysis` is emitted
- **WHEN** holistic analysis completes
- **THEN** event `holistic_done` is emitted
