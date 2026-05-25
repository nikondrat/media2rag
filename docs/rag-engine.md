# RAG Engine

## Pipeline

```
User message
   │
   ├─ (1) Memory Recall (past context)
   ├─ (2) Query Rewrite
   │     ├── Format detection
   │     ├── Semantic rewrite
   │     └── Multi-query expansion
   ├─ (3) HyDE (optional)
   ├─ (4) Hybrid Search (dense + sparse)
   ├─ (5) Parent Lookup
   ├─ (6) Reranking
   ├─ (7) Dedup
   ├─ (8) Context Build
   └─ (9) LLM → answer
```

Каждый этап — отдельная стадия, включается/выключается через конфиг. v1: только Search + Context Build + LLM. Остальное добавляется по мере готовности.

---

## Query Rewrite

**Зачем:** пользователи не умеют формулировать запросы для поиска.

### Format detection (без LLM)

Определяем тип входа по эвристикам:

```go
type InputFormat string

const (
    FormatQuestion  InputFormat = "question"  // "как ...?", "что ...?"
    FormatCommand   InputFormat = "command"   // "напиши ...", "объясни ..."
    FormatStatement InputFormat = "statement" // "я считаю что ..."
    FormatFragment  InputFormat = "fragment"  // "возражения дорого"
)

func DetectFormat(input string) InputFormat {
    // по первому слову, знаку вопроса, длине
}
```

### Semantic rewrite (LLM, 1 вызов)

**Промпт:**
```
Turn this user message into a search query for knowledge base retrieval.
- If it's a question: rephrase as clear, specific search terms
- If it's a command: extract the topic
- If it's a statement: extract the core concept
Output only the rewritten query, 10-20 words.

User: {input}
Search query:
```

### Multi-query expansion (LLM, 1 вызов)

**Промпт:**
```
Generate 3 search queries based on this query. Each should cover a different aspect.
Output one query per line, no numbering.

Query: {rewritten_query}
```

Результат: 3-5 запросов, поиск по каждому, результаты объединяются (RRF).

---

## Hybrid Search

### Dense (семантический)
- Эмбеддинг вопроса → косинусная близость с эмбеддингами чанков
- Хорошо: понимает смысл, синонимы, концепции
- Плохо: теряет точные термины

### Sparse (BM25)
- Частотный анализ слов в чанках
- Хорошо: точные термины, имена, редкие слова
- Плохо: не понимает смысл

### RRF Fusion

Qdrant выполняет оба поиска параллельно (prefetch), затем объединяет через Reciprocal Rank Fusion:

```go
// Score = 1/(k + rank_dense) + 1/(k + rank_sparse)
// k = 60 (константа сглаживания)
```

```
prefetch dense  (topK * 2)    prefetch sparse (topK * 2)
         │                            │
         └────────── RRF ─────────────┘
                        │
                   результат (topK)
```

### Конфиг

```yaml
rag:
  hybrid_search:
    enabled: true
    dense_weight: 0.5     # вес dense в RRF
    sparse_weight: 0.5    # вес sparse
    rrf_k: 60             # константа сглаживания
```

Детали реализации: [vector-store.md](vector-store.md)

---

## Post-processing (TBD)

- Parent Lookup
- Reranking
- Dedup

---

## Context Build (TBD)

Format context from search results → LLM prompt.

---

## Memory (TBD)

Recall past session context.
