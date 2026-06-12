# Hybrid RAG+GraphRAG — семантический графовый поиск

## Проблема

| Команда | Что находит |
|---------|-------------|
| `rag "хочу масштаб"` | Чанки про масштабирование (semantic similarity) |
| `graphrag "хочу масштаб"` | Entity lookup в графе — если exact match не найден, пусто |

Разрыв: semantic search понимает **о чём** запрос, но не видит **связей**.
Graph traversal видит связи, но требует **точного имени** entity.

## Решение: гибрид

```
User: "хочу масштаб"
       │
       ▼
  ┌───────────────────┐
  │ 1. Query Rewriter  │ → entities: ["масштаб", "масштабирование"]
  └───────┬───────────┘
          │
     ┌────┴────┐
     ▼         ▼
  ┌───────┐ ┌────────────┐
  │ RAG   │ │ GraphRAG   │  ← параллельно
  │       │ │            │
  │ Qdrant│ │ Entity     │
  │ search│ │ lookup     │
  └───┬───┘ └─────┬──────┘
      │           │
      ▼           ▼
  Chunks       Nodes found
  (semantic)   (exact + case-insensitive)
      │           │
      │     ┌─────┘
      │     ▼
      │  Graph traversal
      │  (causal chains)
      │           │
      └─────┬─────┘
            ▼
   ┌─────────────────┐
   │ LLM synthesis    │
   │ chunk context    │
   │ + causal chains  │
   │ + provenance     │
   └─────────────────┘
```

### Детали

1. **RAG path:**
   - Qdrant semantic search по всем chunks
   - Выбор топ-K chunks (K=5)
   - Извлечение entity names из найденных чанков

2. **GraphRAG path:**
   - Entity lookup в графе (exact + case-insensitive + alias)
   - Multi-hop BFS traversal (depth=2-3)
   - Causal chains

3. **Hybrid merge:**
   - Chains из graph + контекст из chunks
   - LLM синтезирует ответ: "Вот что говорится (chunks) и как это связано (chains)"

### Когда это нужно

| Запрос | Без гибрида | С гибридом |
|--------|-------------|------------|
| "хочу масштаб" | graphrag: not found | Находит "Масштабирование бизнеса" через similarity |
| "проблемы с сотрудниками" | graphrag: exact name mismatch | RAG находит chunk → entity → graph traversal |
| "как заработать больше" | graphrag: generic entities | RAG подсказывает entity names из контекста |

### Когда НЕ нужно

| Запрос | Почему |
|--------|--------|
| "почему Агрессивные клиенты" | exact match уже работает |
| "какие топ-5 тем" | global search через community summaries |
| "что общего у CRM и продаж" | pattern commonality, оба есть в графе |

## CLI

```bash
# Гибрид (RAG + GraphRAG)
media2rag search "хочу масштаб"

# Чистый RAG
media2rag rag "хочу масштаб"

# Чистый GraphRAG
media2rag graphrag "хочу масштаб"
```

## Влияние

- +1 LLM call: синтез chunk context + chains
- +1 Qdrant query: semantic search для entity resolution
- Качество: драматически лучше для **нечётких** запросов
- Обратная совместимость: rag и graphrag продолжают работать как есть
