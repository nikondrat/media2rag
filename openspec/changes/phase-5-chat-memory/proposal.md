## Why

После RAG Engine (Фаза 4) система отвечает на вопросы с источниками. Но каждый запрос — изолированный. Нужен чат с памятью: сессии, история диалогов, извлечение фактов, бесконечный контекст через суммаризацию. SQLite store для сессий и памяти + Chat Engine для управления диалогами.

## What Changes

- SQLite schema и store (`internal/store/sqlite.go`) — сессии, сообщения, факты
- `internal/chat/` — session management, context window, message history
- `internal/memory/` — fact extraction, recall, CRUD
- `chat` команда — интерактивный терминальный чат с RAG-обогащением
- Context management: последние 5 сообщений полный текст, старше — суммаризация
- Fact extraction после каждого ответа LLM

## Capabilities

### New Capabilities
- `sqlite-store`: SQLite-backed storage for sessions, messages, memory facts
- `chat-session`: session CRUD, message history, context window management
- `memory-store`: fact storage, recall by relevance, CRUD operations
- `context-builder`: assemble chat context (history + memories + RAG sources)
- `chat-command`: interactive terminal chat with readline, streaming, sources display

### Modified Capabilities
- `ask-command`: может использовать сессии для контекста
- `rag-engine`: интегрируется с memory recall

## Impact

- Runtime-зависимость: SQLite (встроенный, через `modernc.org/sqlite` — pure Go)
- Новая команда `chat` с интерактивным режимом
- LLM вызов для суммаризации истории + fact extraction
- Не создаётся: HTTP serve mode, REST API — это фаза 6
