## 1. Domain Types — Split model/types.go

- [x] 1.1 Create `internal/model/extracted.go` with `ExtractedContent`, `Section`, `ExtractedImage`
- [x] 1.2 Create `internal/model/rag_doc.go` with `RAGDocument`, `DocumentMetadata`, `Claim`, `KeyTerm`
- [x] 1.3 Create `internal/model/llm.go` with `ChatRequest`, `ChatResponse`, `Message`, `StreamDelta`, `TypedBlock`
- [x] 1.4 Create `internal/model/event.go` with `Event`
- [x] 1.5 Create `internal/model/memory.go` with `MemoryEntry`
- [x] 1.6 Create `internal/model/errors.go` with sentinel errors
- [x] 1.7 Remove original `internal/model/types.go`
- [x] 1.8 Run `go vet ./...` and `go build ./...` to verify no breakage

## 2. Search Algorithms — Move RRF, KeywordOverlapSearch, TopK to rag

- [x] 2.1 Create `internal/rag/ranking.go` with `RRF()`, `KeywordOverlapSearch()`, `TopK()` using `store.SearchResult`
- [x] 2.2 Remove `RRF()`, `KeywordOverlapSearch()`, `TopK()` from `internal/store/qdrant.go`
- [x] 2.3 Update `internal/rag/engine.go` to call `rag.RRF()` instead of `store.RRF()`
- [x] 2.4 Update `internal/e2e/rag_test.go` imports to use `rag` package for ranking functions
- [x] 2.5 Run `go test ./...` to verify

## 3. VectorStore Interface — Abstract store behind interface

- [x] 3.1 Define `VectorStore` interface in `internal/store/store.go`
- [x] 3.2 Rename `store.Store` to `store.QdrantStore` (implements VectorStore)
- [x] 3.3 Update `rag.Engine` to accept `store.VectorStore` instead of `*store.Store`
- [x] 3.4 Update `service.Qdrant` to return `store.VectorStore` from `EnsureRunning`
- [x] 3.5 Update `cmd/media2rag/main.go` and command files for new return types
- [x] 3.6 Run `go build ./cmd/media2rag` and `go vet ./...`

## 4. HTTP Server — Implement serve mode

- [x] 4.1 Create `internal/api/router.go` with mux, CORS, logging middleware
- [x] 4.2 Create `internal/api/health.go` with `GET /api/health` endpoint
- [x] 4.3 Create `internal/api/process.go` with `POST /api/process` endpoint
- [x] 4.4 Create `internal/api/query.go` with `POST /api/query` endpoint
- [x] 4.5 Create `internal/api/server.go` with `Start()`, graceful shutdown, API key middleware
- [x] 4.6 Rewrite `cmd/media2rag/serve.go` as thin wrapper calling `api.Start()`
- [x] 4.7 Run `go build ./cmd/media2rag` and test `media2rag serve`

## 5. Status — Add Qdrant check

- [x] 5.1 Add Qdrant connectivity check in `cmd/media2rag/status.go` (ping via store.ListCollections with 2s timeout)
- [x] 5.2 Run `go build ./cmd/media2rag` and test `media2rag status`
