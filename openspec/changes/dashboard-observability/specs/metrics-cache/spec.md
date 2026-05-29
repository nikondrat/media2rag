## ADDED Requirements

### Requirement: Metrics cache table
The system SHALL cache aggregated metrics in a `metrics_cache` SQLite table with cache_key, JSON data, computed_at, and ttl_seconds fields.

#### Scenario: Cache entry created
- **WHEN** metrics are computed for the first time
- **THEN** a `metrics_cache` entry is created with the computed JSON data and current timestamp

#### Scenario: Cache entry reused
- **WHEN** metrics are requested and a cache entry exists within TTL
- **THEN** the cached data is returned without recomputation

### Requirement: Cache keys
The system SHALL support the following cache keys: `overview:{days}`, `timeline:{days}`, `metrics:{period}`, `embeddings:{period}`, `feedback:{period}`, and `regressions:{period}:{baseline}`.

#### Scenario: Overview cache
- **WHEN** `GET /api/debug/overview?days=7` is called
- **THEN** the cache key is `overview:7`

#### Scenario: Metrics cache
- **WHEN** `GET /api/debug/metrics?period=7d` is called
- **THEN** the cache key is `metrics:7d`

### Requirement: Cache TTL
The system SHALL use different TTLs per cache key: overview (60s), timeline (120s), metrics (120s), embeddings (300s), feedback (60s), regressions (300s).

#### Scenario: TTL respected
- **WHEN** a cache entry exists but its `computed_at + ttl_seconds` is in the past
- **THEN** the entry is considered expired and data is recomputed

### Requirement: Cache invalidation hints
The SSE events SHALL serve as invalidation hints to the frontend, which may choose to refetch data.

#### Scenario: Pipeline complete triggers refetch
- **WHEN** the frontend receives a `pipeline_complete` SSE event
- **THEN** it invalidates the overview and timeline cache and refetches
