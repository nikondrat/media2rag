## Why

После Chat + Memory (Фаза 5) вся логика работает через CLI. Для GUI, web-клиентов и Telegram-бота нужен HTTP-сервер с REST API и SSE-стримингом. Serve mode — долгоживущий процесс, который экспонирует все операции через HTTP endpoints.

## What Changes

- `internal/api/` — HTTP server, router, handlers, CORS middleware
- REST endpoints: `/api/process`, `/api/query`, `/api/sessions`, `/api/documents`, `/api/memory`
- SSE streaming: `/api/stream/{id}` для process, query, chat responses
- Health endpoint: `/health` — статус Qdrant, Ollama, rdrr
- Auth middleware: API key authentication (опционально)
- `serve` команда — полная реализация вместо заглушки

## Capabilities

### New Capabilities
- `http-server`: Go net/http server with graceful shutdown
- `rest-api`: all REST endpoints for process, query, sessions, documents, memory
- `sse-streaming`: Server-Sent Events for real-time progress and LLM streaming
- `cors-middleware`: CORS headers for web clients
- `auth-middleware`: API key authentication via header
- `health-endpoint`: system health check

### Modified Capabilities
- `serve-command`: полная реализация HTTP server вместо заглушки

## Impact

- Долгоживущий процесс (daemon)
- CORS для web-клиентов
- API key для безопасности (опционально)
- SSE вместо WebSocket (проще, работает через proxy)
- Не создаётся: Telegram bot, web GUI — отдельные проекты
