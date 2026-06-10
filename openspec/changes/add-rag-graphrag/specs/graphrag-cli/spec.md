## ADDED Requirements

### Requirement: GraphRAG CLI command
The system SHALL provide `media2rag graphrag <query>` command for causal search over the knowledge graph.

#### Scenario: Causal search
- **WHEN** `media2rag graphrag "почему компании банкротятся"` is called
- **THEN** causal chains leading to "банкротство" are returned

#### Scenario: Search with depth
- **WHEN** `media2rag graphrag "как снизить издержки" --depth 3` is called
- **THEN** graph traversal goes up to 3 hops from matched entities

### Requirement: Entity extraction from query
The system SHALL extract entities from the user query using LLM.

#### Scenario: Extract entities from query
- **WHEN** query is "какой бизнес запустить в строительстве"
- **THEN** entities extracted: Business("бизнес"), Market("строительство")

### Requirement: Local search (entity fan-out)
The system SHALL perform local search by finding entities and traversing their neighbors.

#### Scenario: Fan-out from entity
- **WHEN** entity "банкротство" is found
- **THEN** incoming edges with relation=causes are traversed to find root causes

#### Scenario: Multi-hop traversal
- **WHEN** depth=2
- **THEN** traversal goes: entity → neighbors → neighbors of neighbors

### Requirement: Global search (community summaries)
The system SHALL answer holistic queries using community summaries.

#### Scenario: Holistic query
- **WHEN** query is "какие топ-5 тем в базе"
- **THEN** community summaries are ranked and top 5 are returned

### Requirement: Path ranking
The system SHALL rank paths by confidence and relevance to query.

#### Scenario: Rank by confidence
- **WHEN** multiple paths exist from A to B
- **THEN** paths with higher confidence relations are ranked first

#### Scenario: Rank by relevance
- **WHEN** paths are found
- **THEN** embedding similarity to query is used as secondary ranking

### Requirement: JSON output with chains and provenance
The GraphRAG command SHALL support `--format json` with causal chains and provenance.

#### Scenario: JSON output with chains
- **WHEN** `media2rag graphrag "query" --format json` is called
- **THEN** output includes: query, entities[], chains[] (path, relations, confidence), opportunities[], provenance[]

### Requirement: Reasoning chain generation
The system SHALL generate a reasoning chain using LLM based on the subgraph.

#### Scenario: Generate reasoning
- **WHEN** subgraph is retrieved
- **THEN** LLM generates answer with explicit reasoning chain: "A causes B, which enables C, therefore..."

### Requirement: DRIFT search
The system SHALL support DRIFT search: local entity fan-out augmented with community context.

#### Scenario: DRIFT search
- **WHEN** query is "как решить X"
- **THEN** fan-out from entity X + community summary provides broader context

### Requirement: Query patterns
The system SHALL support 4 query patterns with automatic detection.

#### Scenario: "Почему X?" (root cause)
- **WHEN** query asks "почему" or "причина"
- **THEN** traverse incoming edges with relation=causes to find root causes

#### Scenario: "Что если убрать X?" (counterfactual)
- **WHEN** query asks "что если убрать" or "без X"
- **THEN** traverse outgoing edges with relation=enables/requires to find what becomes impossible

#### Scenario: "Как достичь X?" (prerequisites)
- **WHEN** query asks "как достичь" or "что нужно для"
- **THEN** traverse incoming edges with relation=requires/enables to find prerequisites

#### Scenario: "Что общего у X и Y?" (commonality)
- **WHEN** query asks "что общего" or "связь между"
- **THEN** find common ancestors and descendants, return intersecting paths

### Requirement: Search mode selection
The system SHALL support `--mode local|global|auto` for search mode selection.

#### Scenario: Local mode
- **WHEN** `--mode local` is specified
- **THEN** only entity fan-out is used

#### Scenario: Global mode
- **WHEN** `--mode global` is specified
- **THEN** only community summaries are used

#### Scenario: Auto mode
- **WHEN** `--mode auto` (default)
- **THEN** system chooses based on query type (holistic → global, specific → local)
