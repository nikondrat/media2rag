## Context

Сейчас `rag/engine.go` принимает `*store.Store` (Qdrant) напрямую. `store/qdrant.go` содержит поисковые алгоритмы (RRF, KeywordOverlapSearch), не относящиеся к персистентности. `model/types.go` — единый файл с типами из 5+ доменов. `serve.go` — заглушка.

## Goals / Non-Goals

**Goals:**
- VectorStore interface, rag зависит от него, store имплементирует
- RRF, KeywordOverlapSearch, TopK перенесены в `rag/`
- model/types.go разбит на логические файлы, никаких циклических импортов
- HTTP server работает: `media2rag serve` поднимает роутер с хендлерами
- `media2rag status` проверяет Qdrant

**Non-Goals:**
- Полный рефакторинг model — только разделение на файлы, без переименования типов
- Чат, память, коучинг — только фундамент для HTTP (сами сессии не реализуем)
- Абстракция LLM клиента — там уже есть интерфейс, менять не надо

## Decisions

**1. VectorStore — интерфейс в `store/`, не отдельный пакет**
- Интерфейс `VectorStore` в `store/store.go`, рядом с имплементацией
- Qdrant имплементация: `store/qdrant.go` (не переименовываем)
- Методы: `UpsertPoints`, `SearchPoints`, `DeletePoints`, `GetPointsByID`, `ScrollByFilter`, `InitCollections`, `ListCollections`, `Close`
- `rag/engine.go` принимает `store.VectorStore`, тесты могут передавать mock

**2. RRF, KeywordOverlapSearch, TopK — в `rag/ranking.go`**
- Новый файл `internal/rag/ranking.go`
- store использует `store.SearchResult` — тип остаётся в store для цикла зависимостей
- `rag/engine.go` вызывает `rag.RRF()` и `rag.KeywordOverlapSearch()` вместо `store.RRF()`
- `store.RRF()`, `store.KeywordOverlapSearch()`, `store.TopK()` — удалить

**3. model/types.go разбить на файлы**
- `model/extracted.go` — ExtractedContent, Section, ExtractedImage
- `model/rag_doc.go` — RAGDocument, DocumentMetadata, Claim, KeyTerm
- `model/llm.go` — ChatRequest, ChatResponse, Message, StreamDelta, TypedBlock
- `model/event.go` — Event
- `model/memory.go` — MemoryEntry
- `model/errors.go` — sentinel errors
- Все в package model — один пакет, несколько файлов. Никаких циклических импортов.

**4. HTTP server — `internal/api/`**
- `api/router.go` — mux + middleware (CORS, logging, recovery)
- `api/process.go` — POST /api/process
- `api/query.go` — POST /api/query (RAG)
- `api/health.go` — GET /api/health
- `api/debug.go` — GET /api/debug/* (заглушки для dashboard)
- `serve.go` (cmd) — тонкая обёртка: парсит флаги, вызывает `api.Start()`

**5. Qdrant в status**
- `cmd/media2rag/status.go` — добавить проверку: gRPC ping на Qdrant
- Использовать `store.ListCollections` с таймаутом 2s

## Risks / Trade-offs

- [Risk] `model/` разделение может сломать неявные импорты если какой-то тип тянул целый пакет → Mitigation: сначала разбить, потом собрать проект
- [Risk] Изменение `store/` API (удаление RRF) затронет тесты → Mitigation: обновить rag_test.go в том же PR
- [Risk] HTTP server без авторизации → Ok для v1 (localhost), добавим флаг `--host` для ограничения
