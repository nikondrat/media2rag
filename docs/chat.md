# Chat — v1 Design

## Сессии

Каждая сессия — отдельный диалог. Хранится в SQLite.

```go
type Session struct {
    ID        string    // UUID
    Title     string    // авто-генерация из первого вопроса
    CreatedAt int64
    UpdatedAt int64
}

type Message struct {
    ID        string    // UUID
    SessionID string
    Role      string    // "user" | "assistant"
    Content   string
    Sources   []Source  // для assistant
    CreatedAt int64
}

type Source struct {
    Ref       int     // [1], [2], ...
    Title     string  // заголовок документа
    DocType   string  // video, markdown, pdf
    Source    string  // URL или путь
    Content   string  // релевантный фрагмент
}
```

## Контекстное окно

```
Вопрос пользователя
   │
   ├─ (1) Memory Recall → релевантные факты
   ├─ (2) Query Rewrite → 3 варианта поиска
   ├─ (3) Hybrid Search → 20 чанков
   ├─ (4) Rerank → 5 лучших
   ├─ (5) Parent Lookup → parent-чанки
   ├─ (6) Dedup → уникальные
   ├─ (7) Context Build → промпт с источниками
   └─ (8) LLM Stream → ответ
```

**Управление контекстом:**
- Последние 5 сообщений — полный текст
- Старше — LLM суммаризует
- RAG контекст — top 5 чанков
- Факты из памяти — top 3

**Пример промпта:**
```
--- System ---
You are a helpful assistant. Answer based on the provided context.
Cite sources using [1], [2], etc.

--- Recent History ---
User: расскажи про HNSW
Assistant: HNSW это графовый индекс... [1]

--- Relevant Memories ---
[1]: "Пользователь интересуется параметрами HNSW"

--- Context ---
Source [1]: "Как масштабировать бизнес" (video, https://youtube.com/...)
> Ключевой принцип масштабирования — делегирование...

Source [2]: "Скрипты продаж" (markdown, ./scripts.md)
> При обработке возражения "дорого"...

--- Question ---
а какие у него параметры?
```

## Стриминг

**Subprocess mode:**
```bash
media2rag chat --session "abc123" "вопрос" --json
```
```json
{"type": "llm_token", "data": {"token": "HNSW"}}
{"type": "llm_token", "data": {"token": " это"}}
{"type": "sources", "data": {"sources": [...]}}
{"type": "completed"}
```

**Serve mode (HTTP):**
```
POST /api/chat
→ 202 Accepted {"session_id": "abc123"}

WS /api/stream/abc123
→ {"type": "token", "data": "HNSW"}
→ {"type": "token", "data": " это"}
→ {"type": "sources", "data": [...]}
→ {"type": "done"}
```

## Конфиг

```yaml
chat:
  history_length: 5        # сообщений в контексте
  context_tokens: 8000     # лимит контекста (токены)
  max_sources: 5           # максимум источников в ответе
  summarize_threshold: 10  # после N сообщений суммаризировать
```
