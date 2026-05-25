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
| pdurugyan/qwen3-reranker-0.6b-q8_0 | 0.6GB |  |  |

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

## Parent Lookup

**Зачем:** ищем в `children` (точные, 128 токенов), а в LLM отправляем `parents` (контекстные, 512 токенов).

### Алгоритм

```go
func (s *Store) ParentLookup(ctx context.Context, results []SearchResult) ([]ParentChunk, error) {
    // 1. Собираем уникальные parent_id из найденных child
    parentIDs := make(map[string]bool)
    for _, r := range results {
        pid := r.Payload["parent_id"].(string)
        if pid != "" {
            parentIDs[pid] = true
        }
    }

    // 2. Запрашиваем parents из Qdrant
    parents, err := s.client.Get(ctx, &qdrant.GetPoints{
        Collection: "parents",
        IDs:        toSlice(parentIDs),
    })
    if err != nil {
        return nil, err
    }

    // 3. Сортируем по количеству child-совпадений
    for _, p := range parents {
        matchCount := countChildMatches(p, results)
        ranked = append(ranked, ParentChunk{
            Content:     p.Payload["content"].(string),
            MatchCount:  matchCount,
            DocumentID:  p.Payload["document_id"].(string),
        })
    }
    sort.Slice(ranked, func(i, j int) bool {
        return ranked[i].MatchCount > ranked[j].MatchCount
    })

    return ranked, nil
}
```

### Принцип

```
child_42: "HNSW m=16"          ─┐
child_45: "ef_construct 200"   ─┼── parent_12 (3 совпадения)
child_47: "оптимизация поиска" ─┘

child_12: "установка Qdrant"   ── parent_05 (1 совпадение)

→ LLM получает: parent_12, parent_05
```

### v2: Parent + child цитаты

```markdown
## Qdrant HNSW параметры
Parent chunk content...

> Цитата: "HNSW m=16 обеспечивает баланс..." (child)
> Цитата: "ef_construct влияет на качество индекса" (child)
```

---

## Dedup

**Зачем:** multi-query возвращает одни и те же чанки несколько раз. Без дедупа контекст забивается дубликатами.

### Как работает

```go
func Dedup(results []SearchResult) []SearchResult {
    seen := map[string]bool{}
    deduped := []SearchResult{}

    for _, r := range results {
        h := r.Payload["content_hash"].(string)
        if seen[h] {
            continue
        }
        seen[h] = true
        deduped = append(deduped, r)
    }

    return deduped
}
```

**До dedup** (5 результатов, 3 уникальных):
```
1. "HNSW m=16"         (из Q1)
2. "ef_construct=200"  (из Q1)
3. "HNSW m=16"         (из Q2) ← дубль
4. "HNSW m=16"         (из Q3) ← дубль
5. "установка Qdrant"  (из Q1)
```

**После dedup** (3 уникальных):
```
1. "HNSW m=16"
2. "ef_construct=200"
3. "установка Qdrant"
```

SHA-256 хеш хранится в payload точки Qdrant (поле `content_hash`), вычисляется при индексации.

---

## Context Build

**Зачем:** собрать найденные чанки в промпт для LLM с источниками, чтобы LLM цитировал откуда взял информацию.

### Формат

```go
type ContextBuilder struct{}

func (b *ContextBuilder) Build(query string, chunks []ParentChunk) []Message {
    var sources []string
    var contextLines []string

    for i, chunk := range chunks {
        ref := i + 1
        source := fmt.Sprintf("[%d]: %s (%s, %s)",
            ref, chunk.Title, chunk.DocType, chunk.Source)
        sources = append(sources, source)

        contextLines = append(contextLines,
            fmt.Sprintf("Source [%d]:", ref),
            fmt.Sprintf("> %s", chunk.Content),
            "")
    }

    context := strings.Join(contextLines, "\n")
    sourceBlock := strings.Join(sources, "\n")

    system := fmt.Sprintf(`You are a helpful assistant. Answer based on the provided context.
Cite sources using [1], [2], etc. If the context doesn't contain the answer, say so.
Sources:
%s`, sourceBlock)

    return []Message{
        {Role: "system", Content: system},
        {Role: "system", Content: "Context:\n" + context},
        {Role: "user", Content: query},
    }
}
```

### Пример

**Контекст:**
```
--- Sources ---
[1]: "Как масштабировать бизнес" (video, https://youtube.com/...)
[2]: "Скрипты продаж" (markdown, ./scripts.md)

--- Context ---
Source [1]:
> Ключевой принцип масштабирования — делегирование.
> Невозможно вырасти, если ты делаешь всё сам.

Source [2]:
> При обработке возражения "дорого" используйте технику "Сэндвич".
```

**Ответ LLM:**
```
Основной принцип масштабирования — делегирование [1].
Для обработки возражений используйте технику "Сэндвич" [2].
```

---

## Memory (TBD)

Recall past session context.
