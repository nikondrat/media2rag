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
