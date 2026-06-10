## ADDED Requirements

### Requirement: Topic-based community clustering
The system SHALL group chunks into communities based on their `topic` field.

#### Scenario: Group by topic
- **WHEN** chunks have topics ["Воронка продаж", "Воронка продаж", "CRM"]
- **THEN** two communities are created: "Воронка продаж" (2 chunks), "CRM" (1 chunk)

### Requirement: Community hierarchy
The system SHALL organize communities into domains using LLM-generated hierarchy.

#### Scenario: Generate domain hierarchy
- **WHEN** communities exist: ["Воронка продаж", "CRM", "Kubernetes", "Docker"]
- **THEN** domains are: {"sales": ["Воронка продаж", "CRM"], "devops": ["Kubernetes", "Docker"]}

### Requirement: Community summary generation
The system SHALL generate an LLM summary for each community, synthesizing key insights from all chunks.

#### Scenario: Generate summary
- **WHEN** community has 5 chunks about "Воронка продаж"
- **THEN** summary contains synthesized insights from all 5 chunks

#### Scenario: Summary includes provenance
- **WHEN** summary is generated
- **THEN** it references source chunk IDs for each claim

### Requirement: Global search via community summaries
The system SHALL answer holistic queries using community summaries.

#### Scenario: Answer "what are top 5 themes"
- **WHEN** query is "какие топ-5 тем в базе"
- **THEN** community summaries are ranked and top 5 are returned with details

### Requirement: Community context for local search
The system SHALL include community summary as context when doing local search from an entity.

#### Scenario: Local search with community context
- **WHEN** query is "почему компании банкротятся"
- **THEN** entity fan-out returns neighbors + community summary provides broader context
