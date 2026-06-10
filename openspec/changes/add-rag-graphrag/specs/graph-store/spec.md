## ADDED Requirements

### Requirement: Graph storage as JSON adjacency list
The system SHALL store the knowledge graph as a JSON adjacency list with nodes and edges.

#### Scenario: Save graph
- **WHEN** graph is built after extraction
- **THEN** `graph.json` is written with nodes[] and edges[] arrays

#### Scenario: Load graph
- **WHEN** `media2rag graphrag` is called
- **THEN** graph is loaded from `graph.json` into memory

### Requirement: Node schema
Each node SHALL have: id, name, type, description, metadata{}, source_chunks[].

#### Scenario: Create node
- **WHEN** entity "Отсутствие CRM" is extracted
- **THEN** node has id (hash), name, type="Problem", description, metadata, source_chunks

### Requirement: Type-specific node fields
Each node type SHALL have specific fields in metadata:

| Type | Specific Fields |
|------|----------------|
| Problem | severity, domain |
| Solution | type, maturity |
| Opportunity | confidence, timeframe |
| Skill | level, relevance |
| Resource | source, type, date |
| Market | size, growth |
| Audience | size, needs |
| Business | type, domain, competitors |
| Event | date, impact, source |
| Claim | confidence, source, verified |
| Metric | value, unit, context |
| Concept | domain, mental_model |

#### Scenario: Problem node with severity
- **WHEN** Problem entity is extracted with severity info
- **THEN** metadata contains {"severity": "high", "domain": "sales"}

### Requirement: Edge schema
Each edge SHALL have: from, to, relation_type, mechanism, confidence, source_chunk.

#### Scenario: Create edge
- **WHEN** relation "causes" is extracted between two entities
- **THEN** edge has from_id, to_id, relation_type="causes", mechanism, confidence, source_chunk

### Requirement: Type-specific edge fields
Each edge type SHALL have specific fields:

| Relation | Specific Fields |
|----------|----------------|
| causes | mechanism |
| enables | condition |
| prevents | reason |
| requires | condition |
| solves | effectiveness |
| blocks | reason |
| competes_with | dimension |
| serves | segment |
| leverages | advantage |
| leads_to | timeframe |
| correlates | strength |
| supports | (none extra) |
| contradicts | explanation |
| part_of | (none extra) |

#### Scenario: Causal edge with mechanism
- **WHEN** "Плохой UX causes churn" is extracted
- **THEN** edge has mechanism="пользователи уходят из-за плохого интерфейса"

### Requirement: Graph indexing
The system SHALL build indexes for fast lookup: by_name, by_type, by_relation.

#### Scenario: Lookup by name
- **WHEN** query contains entity "банкротство"
- **THEN** node is found via name index in O(1)

#### Scenario: Lookup by type
- **WHEN** filter by type="Problem"
- **THEN** all Problem nodes are returned via type index

#### Scenario: Lookup by relation
- **WHEN** find all edges with relation_type="causes"
- **THEN** edges are returned via relation index

### Requirement: Incremental updates
The system SHALL support incremental graph updates via `media2rag index --incremental`.

#### Scenario: Add new document
- **WHEN** new RAGDocument is processed
- **THEN** only new chunks are extracted, graph is merged

### Requirement: Graph validation
The system SHALL validate graph integrity: no orphan edges, valid node types, valid relation types.

#### Scenario: Validate graph
- **WHEN** graph is loaded
- **THEN** validation checks pass or error is returned
