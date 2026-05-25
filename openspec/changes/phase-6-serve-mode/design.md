## Context

После Фазы 5 вся логика работает через CLI. Фаза 6 добавляет HTTP-сервер для GUI, web, Telegram-бот клиентов.

**Constraints:**
- Стандартный `net/http` — no frameworks
- SSE для стриминга (проще WebSocket, работает через proxy)
- CORS для web-клиентов
- API key auth (опционально)
- Graceful shutdown

## Goals / Non-Goals

**Goals:**
- HTTP server с graceful shutdown
- REST API: process, query, sessions, documents, memory
- SSE streaming для process, query, chat
- Health endpoint
- CORS middleware
- Auth middleware (API key)

**Non-Goals:**
- WebSocket — SSE достаточно
- gRPC — REST проще для web
- Web GUI — отдельный проект
- Telegram bot — отдельный проект

## Decisions

### 1. SSE вместо WebSocket
**Why:** Проще, работает через reverse proxy, нативная поддержка в браузерах. Go `http.ResponseWrite` + `flusher`.
**Alternatives considered:** WebSocket — сложнее, требует upgrade, не работает через некоторые proxy.

### 2. net/http без фреймворка
**Why:** Go 1.22+ поддерживает method-based routing (`GET /api/path`). Минимум зависимостей.
**Alternatives considered:** `chi`, `gin` — удобнее, но лишние зависимости для простого API.

### 3. Task-based process endpoint
**Why:** Process — долгая операция. POST возвращает `task_id`, клиент подписывается на SSE stream.
**Alternatives considered:** Blocking response — timeout для больших документов.

### 4. API key через header
**Why:** `Authorization: Bearer <key>` — стандарт. Middleware проверяет перед обработкой.
**Alternatives considered:** JWT — overkill для single-binary server.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Goroutine leak при SSE disconnect | context cancellation, defer cleanup |
| Concurrent process requests | Task queue with max concurrency |
| No TLS in dev | Configurable TLS for production |
