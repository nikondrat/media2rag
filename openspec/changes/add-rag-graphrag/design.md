## Context

Pipeline уже генерирует RAGDocument с chunks, causal chains, metadata. Каждый chunk имеет `type`, `topic`, `summary`, `key_points`, `causal_chains`. Эти данные — готовый источник для Knowledge Graph.

Qdrant spec существует (`openspec/specs/qdrant-store/spec.md`) но implementation не восстановлен из git. Hybrid search spec существует (`openspec/specs/hybrid-search/spec.md`) но не реализован.

Microsoft GraphRAG best practices:
1. LLM-generated knowledge graph из raw text
2. Leiden clustering для community detection
3. Community summaries для global search
4. Local search = entity fan-out + neighbor traversal
5. Provenance — каждая связь имеет source_chunk

## Goals / Non-Goals

**Goals:**
- CLI команды `media2rag rag` и `media2rag graphrag` работают
- Entity extraction из chunks: 12 типов сущностей, 14 типов связей
- Graph storage: JSON adjacency list с indexing
- Community summaries: topic-based clustering + LLM summary
- Local search: 2-3 hop traversal от entity
- Global search: query по community summaries
- JSON output для AI агентов с provenance

**Non-Goals:**
- Полноценная graph DB (Neo4j, FalkorDB) — позже, если нужно
- Leiden clustering — используем topic-based clustering
- Real-time graph updates — batch indexing
- Graph visualization — CLI only
- Memory/user profile — отдельная фаза

## Decisions

1. **JSON adjacency list** vs embedded graph DB (Kuzu)
   - Выбран JSON: проще, без внешних зависимостей, Go-native
   - Миграция на Kuzu позже если нужен query language

2. **Topic-based communities** vs Leiden clustering
   - Выбран topic: chunks с одинаковым `topic` → одно сообщество
   - 80% ценности GraphRAG без сложного graph ML

3. **Fixed ontology (12 entities + 14 relations)** vs free-form LLM extraction
   - Выбран fixed: предсказуемые запросы, AI агент знает что спрашивать
   - LLM добавляет кастомные атрибуты через `metadata`

4. **Local + Global search** vs только local
   - Оба нужны: local для конкретных вопросов, global для holistic analysis
   - Global использует community summaries

5. **Batch indexing** vs real-time updates
   - Выбран batch: `media2rag index` строит граф из всех chunks
   - Real-time позже если нужен streaming

6. **Query Rewriter** vs ручные флаги
   - Выбран Query Rewriter: пользователь пишет natural language, LLM преобразует в structured query
   - Без этого graphrag бесполезен — пользователь не знает как формулировать запросы
   - Pattern detection: "почему X?" → root_cause, "что если убрать X?" → counterfactual

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    GRAPHRAG PIPELINE                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Index Command                                            │
│     media2rag index                                          │
│     → Load all RAGDocuments from workspace                   │
│     → For each chunk: extract entities + relations (LLM)     │
│     → Deduplicate entities (embedding + LLM resolve)         │
│     → Build graph: nodes + edges                             │
│     → Cluster by topic → communities                         │
│     → Generate community summaries (LLM)                     │
│     → Save: graph.json, communities.json                     │
│                                                              │
│  2. RAG Command                                              │
│     media2rag rag "query"                                    │
│     → Embed query → Qdrant search (hybrid: dense + sparse)   │
│     → RRF fusion → topK chunks                               │
│     → Build context → LLM answer                             │
│     → Output: answer + sources (JSON or text)                │
│                                                              │
│  3. GraphRAG Command                                         │
│     media2rag graphrag "почему всё плохо с продажами"        │
│     → QueryRewriter: natural language → structured query     │
│       entities: ["продажи", "конверсия", "воронка"]          │
│       pattern: root_cause, mode: local, depth: 3             │
│     → Local search: fan-out from entities (2-3 hop)          │
│     → Global search: query community summaries               │
│     → DRIFT search: local + community context                │
│     → Merge results → LLM answer with reasoning chains       │
│     → Output: answer + chains + provenance (JSON or text)    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Graph Schema

```
Nodes (12 types):
  Problem, Solution, Opportunity, Skill, Resource,
  Market, Audience, Business, Event, Claim, Metric, Concept

Edges (14 types):
  causes, enables, prevents, requires, solves, blocks,
  competes_with, serves, leverages, leads_to, correlates,
  supports, contradicts, part_of

Each edge has:
  - from, to (node IDs)
  - mechanism/description (string)
  - confidence (float)
  - source_chunk (reference to original chunk)
```

## Data Flow

```
RAGDocument.Chunks
       ↓
EntityExtractor (LLM) → []GraphNode + []GraphEdge
       ↓
EntityResolver (dedup) → merged nodes + edges
       ↓
GraphBuilder → adjacency list
       ↓
CommunityCluster (topic) → communities
       ↓
CommunitySummarizer (LLM) → summaries
       ↓
graph.json + communities.json

User Query (natural language)
       ↓
QueryRewriter (LLM) → structured query
       ↓
GraphEngine → traversal → subgraph
       ↓
LLM reasoning → answer + chains + provenance
```

## Risks / Trade-offs

- **LLM extraction cost** — каждый chunk требует LLM вызова. Решение: batch processing, caching
- **Entity deduplication** — "склады" vs "warehouse" vs "складская недвижимость". Решение: embedding similarity + LLM resolve
- **Graph size** — 200+ файлов × 10 chunks × 5 entities = 10K nodes. JSON adjacency list справится
- **Stale graph** — batch indexing = граф устаревает при добавлении файлов. Решение: `media2rag index --incremental`
- **Query Rewriter accuracy** — LLM может неправильно определить pattern. Решение: fallback на auto-mode + пользователь может переопределить `--mode`
