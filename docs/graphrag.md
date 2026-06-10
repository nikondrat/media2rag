# GraphRAG — Архитектура каузального поиска

## Проблема текущего подхода

Vector RAG находит **семантически похожие** чанки, но не понимает **логические связи** между ними.

```
Query: "Почему компания X обанкротилась?"

Vector search:
  Находит чанки со словами "банкротство", "компания X", "финансы"
  Но НЕ находит чанк про "плохой UX" — семантически далёкий

Результат: поверхностный ответ, без корневых причин
```

## Решение: Knowledge Graph

### Концепция

```
┌─────────────────────────────────────────────────────────────┐
│                    KNOWLEDGE GRAPH                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  [Плохой UX] ──(cause)──► [Churn 40%] ──(cause)──►          │
│                              │                               │
│                              ▼                               │
│                        [Revenue drop] ──(cause)──►           │
│                                                                │
│                              ▼                               │
│                        [Банкротство]                         │
│                                                                │
│  [Отсутствие автоматизации] ──(enables)──► [Масштабирование] │
│                                                                │
│  [Human bottleneck] ──(prevents)──► [Масштабирование]        │
│                                                                │
└─────────────────────────────────────────────────────────────┘
```

### Узлы (Nodes)
- **Concepts** — ключевые идеи, термины, сущности
- **Events** — события, кейсы, примеры
- **Frameworks** — методологии, системы

### Рёбра (Edges)
- **causes** — A вызывает B
- **enables** — A делает B возможным
- **prevents** — A блокирует B
- **requires** — B невозможно без A
- **similar_to** — A и B семантически близки
- **example_of** — A — пример концепта B

## Архитектура (Target)

```
┌─────────────────────────────────────────────────────────────┐
│                    GRAPHRAG PIPELINE                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Stage 1: Structural Extraction                             │
│    → type, topic, summary, key_points                       │
│    → (то, что уже работает)                                 │
│                                                              │
│  Stage 2: Entity & Relation Extraction                      │
│    → Выделение сущностей из chunks                          │
│    → Извлечение связей: [A] → [relation] → [B]              │
│    → causal_chain, preconditions, dependencies              │
│                                                              │
│  Stage 3: Graph Construction                                │
│    → Nodes = concepts/topics                                │
│    → Edges = causal chains, dependencies                    │
│    → Storage: graph DB или adjacency list                   │
│                                                              │
│  Stage 4: Graph-Augmented Query                             │
│    → Query → Graph traversal → Path finding                 │
│    → "Почему X?" → ищем incoming edges к X                  │
│    → "Что если убрать X?" → counterfactual traversal        │
│    → Context = path + surrounding nodes                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Промежуточный шаг: Markdown+

Прежде чем строить полноценный GraphRAG, можно расширить текущий формат:

### Новые поля в chunk frontmatter

```markdown
---
# Существующие
type: framework
topic: Автоматизация продаж
summary: ...
key_points: [...]

# НОВЫЕ — каузальность
causal_chain:
  - cause: "Отсутствие CRM"
    mechanism: "Менеджеры забывают跟进 клиентов"
    effect: "Потеря 30% лидов"

preconditions:
  - "Если компания > 10 сотрудников"
  - "Если цикл сделки > 2 недель"

dependencies:
  - "Автоматизация требует стандартизированного процесса"
  - "CRM бесполезна без определённой воронки"

counterfactuals:
  - "Без автоматизации → масштабирование невозможно"
  - "Если убрать CRM → возврат к ручному контролю"
---
```

### Преимущества Markdown+

| | Markdown+ | Full GraphRAG |
|---|---|---|
| Сложность | Низкая — новые поля в промпт | Высокая — новый storage |
| Causal extraction | ✅ | ✅ |
| Multi-hop queries | ❌ (только linear read) | ✅ |
| Time to implement | 1-2 дня | 2-4 недели |
| Risk | Минимальный | Высокий |

### Путь миграции

```
Сейчас          Markdown+         GraphRAG
(вектор)     ─────────►      ─────────►
             causal fields        graph DB
             в markdown           traversal
```

Markdown+ — это **подготовка данных** для GraphRAG. Когда causal-поля есть в каждом chunk, построить граф = распарсить markdown и построить adjacency list.

## Edge Types (онтология связей)

| Edge | Описание | Пример |
|------|----------|--------|
| `causes` | A непосредственно вызывает B | "Плохой UX → churn" |
| `enables` | A делает B возможным | "API → интеграции" |
| `prevents` | A блокирует B | "Техдолг → фичи" |
| `requires` | B невозможно без A | "Масштабирование требует автоматизации" |
| `correlates` | A и B связаны, но causality неясна | "Revenue и NPS" |
| `example_of` | A — пример B | "Tesla — пример direct-to-consumer" |
| `part_of` | A — компонент B | "Sprint — часть Scrum" |
| `contradicts` | A противоречит B | "Agile vs Waterfall" |

## Query Patterns (GraphRAG)

### "Почему X?"
```
→ Найти узел X
→ Пройти incoming edges с relation=causes
→ Вернуть цепочку причин
```

### "Что если убрать X?"
```
→ Найти узел X
→ Пройти outgoing edges с relation=enables/requires
→ Найти всё, что станет невозможным
```

### "Как достичь X?"
```
→ Найти узел X
→ Пройти incoming edges с relation=requires/enables
→ Вернуть необходимые условия
```

### "Что общего у X и Y?"
```
→ Найти общие ancestors в графе
→ Найти общие descendants
→ Вернуть пересекающиеся пути
```

---

## CLI команды для GraphRAG

### `media2rag rag <query>`

Векторный поиск по chunks (Qdrant).

```bash
# Простой поиск
media2rag rag "как масштабировать бизнес"

# С фильтрами
media2rag rag "метрики продаж" --top 10 --min-score 0.7

# JSON output (для AI агентов)
media2rag rag "проблемы логистики" --format json
```

**Output:**
```json
{
  "results": [
    {
      "chunk_id": "chunk_05",
      "file": "Key Marketing Metrics.md",
      "score": 0.85,
      "topic": "CPL и CPO",
      "summary": "...",
      "key_points": ["..."]
    }
  ]
}
```

### `media2rag graphrag <query>`

Каузальный поиск по Knowledge Graph с обходом на 2-3 hop.

```bash
# Поиск с causal chains
media2rag graphrag "почему компании банкротятся"

# С указанием глубины обхода
media2rag graphrag "как снизить издержки" --depth 3

# JSON output
media2rag graphrag "возможности в логистике" --format json
```

**Output:**
```json
{
  "query": "возможности в логистике",
  "entities": ["логистика", "издержки", "склады"],
  "chains": [
    {
      "path": ["маркетплейсы 32%", "высокие издержки", "локализация", "склады"],
      "relations": ["causes", "enables", "leads_to"],
      "confidence": 0.8
    }
  ],
  "opportunities": [
    {
      "problem": "маржинальность съедается логистикой",
      "solution": "локальные склады",
      "monetization": "аренда/управление"
    }
  ]
}
```

---

## Универсальность GraphRAG

GraphRAG — **не только для бизнеса**. Это универсальный инструмент для каузального поиска.

### Примеры использования

**Бизнес-анализ (один из навыков):**
```
Query: "какой бизнес запустить в строительстве"
→ Обход графа: строительство → АЗС → склады → логистика
→ Цепочка: "Wildberries 32%" → "издержки" → "локализация" → "склады"
→ Ответ: бизнес на аренде холодильных складов
```

**Технический анализ:**
```
Query: "почему система падает под нагрузкой"
→ Обход графа: нагрузка → БД → индексы → блокировки
→ Цепочка: "нет индексов" → "full scan" → "блокировки" → "timeout"
→ Ответ: добавить индексы, оптимизировать запросы
```

**Обучение:**
```
Query: "как работает Kubernetes"
→ Обход графа: pod → node → cluster → scheduling
→ Цепочка: "deployment" → "replicaset" → "pod" → "container"
→ Ответ: объяснение с causal links между компонентами
```

---

## Примеры цепочек 2-3 порядка

### Цепочка 1: От проблемы к бизнес-возможности

```
[Wildberries забирает 32%]
    ↓ causes
[высокие издержки на логистику]
    ↓ enables
[выгодно локализовать поставки]
    ↓ leads_to
[спрос на склады-холодильники рядом с городом]
    ↓ opportunity
[бизнес на аренде/управлении складами]
```

### Цепочка 2: Вторичные эффекты

```
[рост маркетплейсов]
    ↓ causes
[рост продаж через интернет]
    ↓ causes
[рост нагрузки на логистику]
    ↓ causes
[дефицит складских помещений]
    ↓ opportunity
[инвестиции в складскую недвижимость]
```

### Цепочка 3: Технический долг

```
[нет тестов]
    ↓ causes
[баги в production]
    ↓ causes
[hotfixes в выходные]
    ↓ causes
[выгорание команды]
    ↓ prevents
[развитие продукта]
```

---

## Сообщества (Communities) — Microsoft GraphRAG Best Practice

### Что это

После извлечения сущностей и связей, граф автоматически кластеризуется алгоритмом **Leiden** (hierarchical community detection).

```
Граф:
  [Проблема X] → [Решение Y] → [Рынок Z]
  [Проблема A] → [Решение B] → [Рынок Z]
  [Технология C] → [Решение Y]

Leiden clustering:
  Community 0 (синий): {Проблема X, Решение Y, Технология C}
  Community 1 (красный): {Проблема A, Решение B, Рынок Z}
  Community 2 (зелёный): {Рынок Z, Рынок D, Рынок E}
```

### Зачем

| Без communities | С communities |
|-----------------|---------------|
| Только local search ("что связано с X?") | + Global search ("какие топ-5 тем?") |
| Нет holistic understanding | LLM summary для каждого кластера |
| Query = entity lookup | Query = community + entity lookup |

### Как работает

1. **Build graph** → entities + relations из chunks
2. **Leiden clustering** → иерархическая группировка (L0, L1, L2...)
3. **Generate summaries** → LLM генерирует summary для каждого community
4. **Query time:**
   - **Global search:** использует community summaries для holistic вопросов
   - **Local search:** fan-out от entity к соседям + community context

### Наш подход: ДА, с реальной пользой

**Реальная польза communities:**
1. **Global search** — "какие топ-5 тем в базе?" → невозможно без community summaries
2. **Holistic understanding** — summary кластера даёт контекст, которого нет в отдельных chunks
3. **Query routing** — по community определяем область знаний вопроса

**Реализация (упрощённая, 80% ценности):**
1. **Community = topic** — chunks с одинаковым `topic` → одно сообщество
2. **LLM summary** — генерируем summary для каждого topic-cluster
3. **Иерархия** — `topics` → `domains` (LLM-генерация)

Без этого: только local search ("что связано с X?")
С этим: + global search ("какие топ-5 тем?")

---

## Storage: Graph DB

### Варианты

| DB | Плюсы | Минусы | Выбор |
|----|-------|--------|-------|
| Neo4j | Мощный Cypher, визуализация | Тяжёлый, JVM | ❌ |
| FalkorDB | Redis-based, быстрый | Молодой | ⚠️ |
| Kuzu | Embedded, Go-friendly | Нет UI | ✅ |
| Adjacency list (JSON) | Просто, без зависимостей | Нет query language | ⚠️ |

**Предварительный выбор:** Kuzu (embedded) или JSON adjacency list (для начала).

### Схема графа

```
Node types (12 — фиксированное ядро + гибрид):

Базовые 8 (бизнес + универсальные):
- Problem: {id, name, description, severity, domain}
- Solution: {id, name, description, type, maturity}
- Opportunity: {id, name, description, confidence, timeframe}
- Skill: {id, name, description, level, relevance}
- Resource: {id, name, source, type, date, chunk_ref}
- Market: {id, name, description, size, growth}
- Audience: {id, name, description, size, needs}
- Business: {id, name, type, domain, competitors}

Добавлены из GraphRAG best practices:
- Event: {id, name, description, date, impact, source}
- Claim: {id, statement, confidence, source, verified}
- Metric: {id, name, value, unit, context}
- Concept: {id, name, description, domain, mental_model}

Edge types (14 — фиксированные):
- causes: {from, to, mechanism, confidence, source_chunk}
- enables: {from, to, condition, confidence, source_chunk}
- prevents: {from, to, reason, confidence, source_chunk}
- requires: {from, to, condition, confidence, source_chunk}
- solves: {from, to, effectiveness, confidence, source_chunk}
- blocks: {from, to, reason, confidence, source_chunk}
- competes_with: {from, to, dimension, source_chunk}
- serves: {from, to, segment, source_chunk}
- leverages: {from, to, advantage, source_chunk}
- leads_to: {from, to, timeframe, confidence, source_chunk}
- correlates: {from, to, strength, source_chunk}
- supports: {from, to, confidence, source_chunk}
- contradicts: {from, to, explanation, source_chunk}
- part_of: {from, to, source_chunk}
```

### Provenance (из GraphRAG best practices)

Каждая связь хранит `source_chunk` — ссылку на оригинальный chunk. Это позволяет:
- Верифицировать ответ ("откуда это?")
- Audit LLM output
- Показать пользователю источник

---

## Query Engine

### Global vs Local Search (GraphRAG best practice)

**Local Search** — ответ на конкретный вопрос:
```
Query: "почему компании банкротятся"
→ Найти entity "банкротство"
→ Fan-out к соседям (incoming: causes)
→ Вернуть цепочку причин + community context
```

**Global Search** — holistic вопрос по всей базе:
```
Query: "какие топ-5 тем в базе"
→ Использовать community summaries
→ LLM ранжирует и агрегирует
→ Вернуть топ-5 тем с provenance
```

**DRIFT Search** (GraphRAG extension):
```
Local search + community context
→ "как решить X" → fan-out от X + summary сообщества
→ Более релевантный ответ с контекстом
```

### Алгоритм обхода графа

```
1. Entity extraction из query
   → "какой бизнес в строительстве"
   → entities: ["бизнес", "строительство"]

2. Graph lookup
   → Найти узлы: "бизнес", "строительство"
   → Найти связанные: "АЗС", "склады", "логистика"

3. Multi-hop traversal (depth=2-3)
   → Для каждого узла: incoming + outgoing edges
   → Построить paths до depth 3

4. Path ranking
   → По confidence relations
   → По relevance к query (embedding similarity)

5. Subgraph → LLM context
   → 10-15 узлов с causal chains
   → Prompt: "построй цепочку рассуждений"

6. Output
   → Answer + reasoning chain + sources
```

### Фильтрация по контексту (будущее)

Если вернёмся к memory/profile:
```
User profile:
  skills: [Go, Python, LLM]
  gaps: [регуляторика пищевых продуктов]
  
Filter:
  - исключить chains с "пищевая лицензия"
  - приоритет chains с "автоматизация"
```

---

## Гибридный подход: Fixed Ontology + LLM Flexibility

### Фиксированное ядро (предсказуемость)
- 12 типов сущностей
- 14 типов связей
- Предсказуемые запросы
- AI агент знает что спрашивать

### LLM добавляет (гибкость)
- Кастомные атрибуты к сущностям (`metadata: {key: value}`)
- Контекстные метки (`tags: ["urgent", "trending"]`)
- Временные метки (`news_date`, `relevance_decay`)
- Новые связи через `correlates` с описанием

### Почему гибрид
1. Бизнес-цикл требует предсказуемой структуры для аналитики
2. Новости/ниши могут требовать новых связей
3. Временной аспект критичен (новость устаревает, навык остаётся)
4. Единый формат для бизнеса, IT, саморазвития

---

## План реализации

### Phase 1: Entity & Relation Extraction (1-2 недели)
- [ ] LLM prompt для извлечения сущностей из chunks
- [ ] Дедупликация сущностей (embedding similarity + LLM resolve)
- [ ] Сохранение в JSON adjacency list

### Phase 2: Graph Storage (1 неделя)
- [ ] Выбор: JSON adjacency list → Kuzu (embedded)
- [ ] Schema migration
- [ ] Indexing для быстрого lookup

### Phase 3: Community Detection (1 неделя)
- [ ] Простая кластеризация по topic
- [ ] LLM summary для каждого community
- [ ] Иерархия: topics → domains

### Phase 4: Query Engine (2 недели)
- [ ] Local search (entity fan-out)
- [ ] Global search (community summaries)
- [ ] CLI команды: `media2rag rag`, `media2rag graphrag`

### Phase 5: AI Agent Integration (1 неделя)
- [ ] JSON output format
- [ ] Provenance в ответах
- [ ] Integration с Hermes и другими агентами

---

## Лучшие практики из Microsoft GraphRAG

1. **Provenance** — каждая связь имеет source_chunk
2. **Community summaries** — holistic search поверх entity lookup
3. **Hierarchical clustering** — уровни абстракции (L0, L1, L2)
4. **Global + Local search** — разные query patterns
5. **LLM-generated graph** — не ручной, а извлечённый из текста
6. **Faithfulness** — SelfCheckGPT для верификации ответов
7. **Prompt tuning** — fine-tune prompts под свой домен
