# GraphRAG — Эволюция (Future Roadmap)

## Текущее состояние (Phase 2)

- **Storage:** JSON adjacency list
- **Communities:** topic-based clustering + **Leiden** (graph-based)
- **Query:** entity extraction + fan-out + community summaries + **Query Rewriter**
- **Export:** GraphML, DOT, JSON formats

---

## Эволюция 1: Leiden Clustering

### Зачем

Topic-based группирует chunks по **одному полю**. Leiden находит сообщества по **структуре графа** — даже если topics разные.

**Пример:**
```
Topic "CRM" chunk: "нет CRM → потеря лидов"
Topic "Воронка" chunk: "плохая воронка → churn"
Topic "Автоматизация" chunk: "автоматизация → рост конверсии"

Topic-based: 3 разных сообщества
Leiden: 1 сообщество "Проблемы продаж" (потому что граф связывает их)
```

### Когда добавлять

Когда заметишь: "у меня chunks с разными topics но они про одно и то же, и граф это показывает".

### Реализация

✅ **Реализовано:**

```bash
media2rag index --cluster leiden    # Leiden вместо topic-based
```

Формат `communities.json` не меняется, только алгоритм clustering.
Go-native implementation в `internal/graph/leiden.go`.

---

## Эволюция 2: Полноценная Graph DB (Kuzu)

### Зачем

JSON adjacency list работает до ~100K nodes. Когда граф растёт:

- Нужен query language (Cypher-like)
- Нужны индексы на диске (не в памяти)
- Нужны concurrent reads/writes без mutex
- Нужна incremental update без full rebuild

### Почему Kuzu

| | Kuzu | Neo4j | FalkorDB |
|---|------|-------|----------|
| Embedded | ✅ | ❌ (JVM) | ❌ (Redis) |
| Go-native | ✅ (CGo) | ❌ | ❌ |
| Cypher-like | ✅ | ✅ | ✅ |
| Single binary | ✅ | ❌ | ❌ |
| Production-ready | ✅ (v0.7+) | ✅ | ⚠️ |

### Миграция

✅ **Подготовлено:** `GraphStore` interface (`internal/graph/store_interface.go`) абстрагирует storage layer.
JSON implementation — default. Kuzu implementation можно добавить без изменения остального кода.

```
JSON adjacency list → Kuzu import → Cypher queries
```

Формат nodes/edges не меняется — только storage layer.

### Когда добавлять

- Граф > 50K nodes
- Нужны сложные запросы: "найди все paths от A до B с confidence > 0.7"
- Нужен concurrent access (server mode)

---

## Эволюция 3: Query Rewriter (предобработчик запросов)

### Проблема

Пользователь пишет: "почему всё плохо с продажами"
Нужно: entities ["продажи"], pattern "why", relations ["causes", "prevents"]

### Решение

LLM-предобработчик перед graph traversal:

```
User query: "почему всё плохо с продажами"
       ↓ Query Rewriter (LLM)
Structured query: {
  "entities": ["продажи", "конверсия", "воронка"],
  "pattern": "root_cause",
  "relations": ["causes", "prevents", "blocks"],
  "mode": "local",
  "depth": 3
}
       ↓ Graph Engine
Fan-out from entities → traversal → reasoning chain
```

### Query Rewriter умеет:

1. **Entity extraction** — извлекает сущности из natural language
2. **Pattern detection** — определяет тип запроса:
   - "почему X?" → root_cause (incoming causes)
   - "что если убрать X?" → counterfactual (outgoing enables/requires)
   - "как достичь X?" → prerequisites (incoming requires/enables)
   - "что общего у X и Y?" → commonality (common ancestors/descendants)
   - "какие топ-5 тем?" → global (community summaries)
   - "как решить X?" → drift (local + community context)
3. **Entity resolution** — "продажи" → ["продажи", "конверсия", "воронка"] (aliases)
4. **Mode selection** — auto: local vs global vs drift
5. **Depth estimation** — простой запрос → depth=2, сложный → depth=3

### Реализация

```bash
# Пользователь пишет как есть
media2rag graphrag "почему всё плохо с продажами"

# Query Rewriter автоматически:
# 1. Извлекает entities
# 2. Определяет pattern = root_cause
# 3. Выбирает mode = local, depth = 3
# 4. Запускает graph traversal
# 5. Возвращает causal chains
```

### Когда добавлять

✅ **Реализовано в Phase 2.** Query Rewriter работает автоматически при вызове `media2rag graphrag`.

---

## Эволюция 4: Real-time Graph Updates

### Сейчас
✅ **Реализовано:** `media2rag update <file.md>` — incremental graph update

```
New document processed
       ↓
Extract new entities + edges
       ↓
Merge into existing graph (dedup, update)
       ↓
Update affected communities
```

### Команды

```bash
# Single file update
media2rag update ./new-doc.md

# Multiple files
media2rag update ./doc1.md ./doc2.md

# Stats
media2rag graph info
```

### Архитектура

`IncrementalUpdater` (`internal/graph/incremental.go`) — dedup nodes by ID, merge source chunks, resolve new entities, update communities.

### Когда добавлять

✅ **Готово к использованию.**

---

## Эволюция 5: Graph Visualization

### Зачем

Увидеть граф глазами, найти паттерны, поделиться.

### Варианты

- ✅ **CLI Export:** `media2rag export --format graphml` → Neo4j import
- ✅ **DOT Export:** `media2rag export --format dot --output graph.dot` → Graphviz
- ✅ **JSON Export:** `media2rag export --format json --output graph.json`
- **Web UI:** `media2rag serve --graph-ui` → localhost:8080/graph (TODO)

### Когда добавлять

После того как граф стабилен и полезен.

---

## Приоритет эволюции

```
✅ Сейчас (Phase 2):
  JSON adjacency + GraphStore interface (Kuzu-ready)
  topic/Leiden communities + Query Rewriter
  Graph Export (GraphML, DOT, JSON)
  Real-time updates: media2rag update <file.md>
  Graph stats: media2rag graph info

Следующий (Phase 3):
  Kuzu Graph DB implementation
  Web Graph UI

Потом (Phase 4):
  Advanced Cypher-like queries
  Concurrent access (server mode)

Будущее (Phase 5):
  Advanced analytics + Visualization enhancements
```
