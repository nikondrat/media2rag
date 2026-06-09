## ADDED Requirements

### Requirement: VectorStore interface defines store contract

The system SHALL define a `VectorStore` interface in `internal/store/store.go` with methods for all current Qdrant operations used by `rag/`.

#### Scenario: Interface covers all RAG operations
- **WHEN** `rag/engine.go` uses a store
- **THEN** it SHALL depend on `store.VectorStore` interface, not `*store.Store`

#### Scenario: Interface methods match current usage
- **WHEN** checking `rag/` and `service/` usage
- **THEN** interface SHALL include: `InitCollections`, `UpsertPoints`, `SearchPoints`, `DeletePoints`, `GetPointsByID`, `ScrollByFilter`, `ListCollections`, `Close`

### Requirement: Existing QdrantStore implements VectorStore

`store.QdrantStore` (rename from `Store`) SHALL implement `store.VectorStore`.

#### Scenario: All interface methods are implemented
- **WHEN** compiling the project
- **THEN** `*store.QdrantStore` SHALL satisfy `store.VectorStore`

### Requirement: Rag engine accepts VectorStore

`rag.Engine` constructor SHALL accept `store.VectorStore` instead of `*store.Store`.

#### Scenario: Engine works with mock store
- **WHEN** creating `rag.NewEngine` with a mock `VectorStore`
- **THEN** it SHALL compile and work without Qdrant running

### Requirement: Service layer uses VectorStore

`service.Qdrant.EnsureRunning` SHALL accept `store.VectorStore` and return it.

#### Scenario: Service passes interface to engine
- **WHEN** `service.Qdrant` initializes the store
- **THEN** it SHALL return a `store.VectorStore`, not a concrete type
