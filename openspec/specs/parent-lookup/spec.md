## ADDED Requirements

### Requirement: Parent chunk lookup
The system SHALL replace child search results with their parent chunks for richer context.

#### Scenario: Lookup parents
- **WHEN** child results have parent_ids
- **THEN** parent chunks are fetched from Qdrant

#### Scenario: Rank by match count
- **WHEN** multiple children reference same parent
- **THEN** parent is ranked by number of child matches
