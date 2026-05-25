## Context

После Фазы 4 есть RAG Engine и `ask` команда. Фаза 5 добавляет чат с памятью: сессии, история, факты, бесконечный контекст.

**Constraints:**
- SQLite через pure Go driver (`modernc.org/sqlite`) — no CGo
- История: последние 5 сообщений полный текст, старше — суммаризация
- Факты извлекаются LLM после каждого ответа
- Факты хранятся в Qdrant (отдельная коллекция `memories`)

## Goals / Non-Goals

**Goals:**
- SQLite store: sessions, messages, facts
- Chat session management: create, load, context build
- Memory: store, recall, delete facts
- `chat` команда: интерактивный терминальный чат
- Context window: history + memories + RAG sources

**Non-Goals:**
- HTTP serve mode — фаза 6
- Coaching engine — отдельная фаза
- Web GUI — отдельный проект

## Decisions

### 1. SQLite через modernc.org/sqlite (pure Go)
**Why:** No CGo, кросс-компиляция работает, zero runtime deps.
**Alternatives considered:** `mattn/go-sqlite3` — требует CGo, ломает кросс-компиляцию.

### 2. Факты в Qdrant, не SQLite
**Why:** Факты ищутся по semantic similarity — нужны embeddings. Qdrant уже есть.
**Alternatives considered:** SQLite FTS — не понимает семантику, только keywords.

### 3. Context window: 5 сообщений + summary
**Why:** Баланс между контекстом и токенами. Суммаризация экономит ~70% токенов.
**Alternatives considered:** sliding window — теряет важные ранние факты.

### 4. Terminal chat: readline + streaming
**Why:** Простой интерактивный режим. `bufio.Scanner` для input, goroutine для stream.
**Alternatives considered:** `bubbletea` — красивее, но лишняя зависимость для v1.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| SQLite corruption | WAL mode, backup |
| Fact extraction noise | Max 3 facts per message, LLM prompt tuning |
| Context overflow | Summarize old messages, token counting |
