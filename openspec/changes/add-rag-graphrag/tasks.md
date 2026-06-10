## 1. Qdrant Restore & Indexing

- [x] 1.1 Restore Qdrant client from git history or re-implement from spec
- [x] 1.2 Implement `media2rag index` command: load all RAGDocuments from workspace
- [x] 1.3 Index chunks into Qdrant: embed + upsert with payload (content, doc_id, parent_id, topic)
- [ ] 1.4 Verify: `media2rag index` processes all documents, Qdrant has points

## 2. Graph Model Layer

- [x] 2.1 Create `internal/model/graph.go` with GraphNode struct (id, name, type, description, metadata, source_chunks)
- [x] 2.2 Create GraphEdge struct (from, to, relation_type, mechanism, confidence, source_chunk)
- [x] 2.3 Create KnowledgeGraph struct (nodes, edges, indexes)
- [x] 2.4 Create Community struct (id, topic, domain, summary, member_chunk_ids)
- [x] 2.5 Define constants: 12 entity types, 14 relation types

## 3. Entity & Relation Extraction

- [x] 3.1 Create `internal/graph/extractor.go` with EntityExtractor
- [x] 3.2 Implement LLM prompt for entity extraction (12 types)
- [x] 3.3 Implement LLM prompt for relation extraction (14 types)
- [x] 3.4 Add batch processing with concurrency control
- [x] 3.5 Add progress bar for extraction
- [ ] 3.6 Verify: test extraction on sample chunks

## 4. Entity Deduplication

- [x] 4.1 Create `internal/graph/resolver.go` with EntityResolver
- [x] 4.2 Implement embedding similarity check (threshold 0.85)
- [x] 4.3 Implement LLM resolution for ambiguous cases (0.7-0.85)
- [x] 4.4 Merge entities with alias tracking
- [ ] 4.5 Verify: "склады" + "warehouse" merge correctly

## 5. Graph Storage

- [x] 5.1 Create `internal/graph/store.go` with JSON adjacency list storage
- [x] 5.2 Implement SaveGraph(graph, path) → graph.json
- [x] 5.3 Implement LoadGraph(path) → *KnowledgeGraph
- [x] 5.4 Build indexes: by_name, by_type, by_relation
- [x] 5.5 Implement graph validation (no orphan edges, valid types)
- [ ] 5.6 Verify: save + load roundtrip, indexes work

## 6. Community Detection & Summaries

- [x] 6.1 Create `internal/graph/communities.go` with topic-based clustering
- [x] 6.2 Group chunks by topic → communities
- [x] 6.3 Implement LLM summary generation per community
- [x] 6.4 Implement domain hierarchy (LLM-generated)
- [x] 6.5 Save communities to communities.json
- [ ] 6.6 Verify: communities are coherent, summaries are useful

## 7. Query Rewriter

- [x] 7.1 Create `internal/graph/rewriter.go` with QueryRewriter
- [x] 7.2 Implement LLM prompt for entity extraction from natural language query
- [x] 7.3 Implement pattern detection (root_cause, counterfactual, prerequisites, commonality, global, drift)
- [x] 7.4 Implement entity resolution (alias lookup + embedding similarity)
- [x] 7.5 Implement mode auto-selection (local/global/drift)
- [x] 7.6 Implement depth estimation (simple=2, complex=3)
- [x] 7.7 Output structured query: {entities[], pattern, relations[], mode, depth}
- [ ] 7.8 Verify: "почему всё плохо с продажами" → correct structured query

## 8. RAG CLI Command

- [x] 8.1 Create `cmd/media2rag/rag.go` with `rag` subcommand
- [x] 8.2 Implement hybrid search: dense + sparse + RRF fusion
- [x] 8.3 Add `--top`, `--min-score`, `--format` flags
- [x] 8.4 Implement text output format
- [x] 8.5 Implement JSON output format
- [ ] 8.6 Verify: `media2rag rag "query"` returns relevant chunks

## 9. GraphRAG CLI Command

- [x] 9.1 Create `cmd/media2rag/graphrag.go` with `graphrag` subcommand
- [x] 9.2 Integrate QueryRewriter: natural language → structured query
- [x] 9.3 Implement local search: entity fan-out, 2-3 hop traversal
- [x] 9.4 Implement global search: community summary ranking
- [x] 9.5 Implement DRIFT search: local + community context
- [x] 9.6 Implement 4 query patterns: root_cause, counterfactual, prerequisites, commonality
- [x] 9.7 Implement auto mode selection (local vs global vs drift)
- [x] 9.8 Add `--depth`, `--mode`, `--format` flags
- [x] 9.9 Implement path ranking (confidence + relevance)
- [x] 9.10 Implement LLM reasoning chain generation
- [x] 9.11 Implement JSON output with chains + provenance
- [ ] 9.12 Verify: `media2rag graphrag "query"` returns causal chains

## 10. Integration & Testing

- [x] 10.1 Wire `media2rag index` → extraction → graph build → save
- [x] 10.2 Wire `media2rag rag` → Qdrant search → output
- [x] 10.3 Wire `media2rag graphrag` → query rewriter → graph lookup → traversal → output
- [ ] 10.4 Test with existing RAGDocuments from workspace
- [ ] 10.5 Test JSON output format for AI agent compatibility
- [ ] 10.6 Verify: end-to-end flow works on 10+ documents

## 11. Documentation

- [x] 10.1 Update `docs/strategy.md` with Phase 2 completion
- [ ] 10.2 Add usage examples to README
- [ ] 10.3 Document JSON output format for AI agents
