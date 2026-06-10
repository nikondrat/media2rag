## ADDED Requirements

### Requirement: Query Rewriter (LLM preprocessor)
The system SHALL preprocess user queries via LLM to extract structured query parameters before graph traversal.

#### Scenario: Natural language query
- **WHEN** user types "почему всё плохо с продажами"
- **THEN** rewriter extracts entities: ["продажи", "конверсия", "воронка"]

#### Scenario: Query with implicit entities
- **WHEN** user types "как масштабироваться"
- **THEN** rewriter resolves to entities: ["масштабирование", "автоматизация", "процессы"]

### Requirement: Pattern detection
The Query Rewriter SHALL detect query pattern and map to graph traversal strategy.

#### Scenario: Root cause pattern
- **WHEN** query contains "почему", "причина", "из-за чего"
- **THEN** pattern = root_cause → traverse incoming edges with relation=causes

#### Scenario: Counterfactual pattern
- **WHEN** query contains "что если убрать", "без", "если не"
- **THEN** pattern = counterfactual → traverse outgoing edges with relation=enables/requires

#### Scenario: Prerequisites pattern
- **WHEN** query contains "как достичь", "что нужно для", "как сделать"
- **THEN** pattern = prerequisites → traverse incoming edges with relation=requires/enables

#### Scenario: Commonality pattern
- **WHEN** query contains "что общего", "связь между", "как связаны"
- **THEN** pattern = commonality → find common ancestors/descendants

#### Scenario: Global pattern
- **WHEN** query contains "какие темы", "топ-", "обзор", "что есть"
- **THEN** pattern = global → use community summaries

#### Scenario: DRIFT pattern
- **WHEN** query contains "как решить", "что делать с"
- **THEN** pattern = drift → local fan-out + community context

### Requirement: Entity resolution
The Query Rewriter SHALL resolve user terms to graph entities using aliases and embedding similarity.

#### Scenario: Alias resolution
- **WHEN** user types "продажи"
- **THEN** resolved to graph entities: ["продажи", "конверсия", "воронка продаж"]

#### Scenario: Fuzzy match
- **WHEN** user types "склады" but graph has "складская недвижимость"
- **THEN** embedding similarity resolves to correct entity

### Requirement: Structured query output
The Query Rewriter SHALL output a structured query for the graph engine.

#### Scenario: Structured query
- **WHEN** user query is processed
- **THEN** output is: {entities[], pattern, relations[], mode, depth}

### Requirement: Mode auto-selection
The Query Rewriter SHALL auto-select search mode (local/global/drift) based on query pattern.

#### Scenario: Auto-select local
- **WHEN** pattern is root_cause or counterfactual
- **THEN** mode = local

#### Scenario: Auto-select global
- **WHEN** pattern is global
- **THEN** mode = global

#### Scenario: Auto-select drift
- **WHEN** pattern is drift
- **THEN** mode = drift

### Requirement: Depth estimation
The Query Rewriter SHALL estimate traversal depth based on query complexity.

#### Scenario: Simple query
- **WHEN** query is specific ("почему churn высокий")
- **THEN** depth = 2

#### Scenario: Complex query
- **WHEN** query is broad ("почему бизнес не растёт")
- **THEN** depth = 3
