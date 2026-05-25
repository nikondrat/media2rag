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

## Reranking

**Зачем:** hybrid search находит 20-50 чанков, но не все релевантны вопросу. Reranker заново оценивает каждый чанк и отсеивает нерелевантные.

### Принцип

Cross-encoder модель получает на вход пару (вопрос + чанк) и возвращает score релевантности:

```
Вопрос: "как настроить HNSW"

До реранжа:                  После:
1. история Qdrant (0.89)     1. параметры HNSW (0.95)  ← реально релевантно
2. параметры HNSW (0.72)     2. настройка индекса (0.82)
3. установка Qdrant (0.68)   3. история Qdrant (0.12)   ← dense ошибся
```

### Реализация

Ollama поддерживает cross-encoder через `/api/rerank`:

```go
type Reranker struct {
    model   string  // bge-reranker-v2-m3
    ollama  *OllamaClient
}

func (r *Reranker) Rerank(ctx context.Context, query string, chunks []Chunk, topK int) ([]ScoredChunk, error) {
    for _, chunk := range chunks {
        score, err := r.ollama.Rerank(ctx, r.model, query, chunk.Content)
        if err != nil {
            return nil, err
        }
        scored = append(scored, ScoredChunk{Chunk: chunk, Score: score})
    }

    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    return scored[:min(topK, len(scored))], nil
}
```

**Запрос к Ollama:**
```
POST /api/rerank
{
  "model": "bge-reranker-v2-m3",
  "query": "...",
  "documents": ["..."],
  "limit": 5
}
```

### Модели

| Модель | Размер | Языки | Скорость |
|--------|--------|-------|----------|
| bge-reranker-v2-m3 | 1.2GB | Русский, английский, китайский | Быстрая |
| ms-marco-MiniLM-L-12-v2 | 500MB | Английский | Очень быстрая |
| jina-reranker-v2-base | 2.5GB | Мультиязычная (50+) | Средняя |

### Конфиг

```yaml
rag:
  reranker:
    enabled: true
    model: bge-reranker-v2-m3     # любая cross-encoder
    top_k: 5                        # сколько оставить
    search_factor: 2                # сколько взять из поиска до реранжа (topK * factor)
```

Reranker не хардкодит язык — модель просто оценивает релевантность на том языке, на котором обучалась.

---

## Context Build (TBD)

Format context from search results → LLM prompt.

---

## Memory (TBD)

Recall past session context.
