## Why

Три реальные архитектурные проблемы мешают тестировать, расширять и поддерживать media2rag:

1. **Нет абстракции VectorStore** — `rag/engine.go` зависит от `store.Store` (конкретная Qdrant имплементация). Нельзя написать юнит-тест без Qdrant, нельзя заменить сторадж без переписывания RAG engine.
2. **store/qdrant.go содержит search-логику** — RRF, KeywordOverlapSearch лежат в пакете для персистентности, а не для ранжирования. При добавлении новых алгоритмов придётся править store.
3. **model/types.go — свалка доменов** — ExtractedContent, ChatRequest, Event, MemoryEntry, TypedBlock, ошибки — всё в одном файле. Нет границ между контекстами.

Плюс `serve.go` — заглушка, HTTP API не работает.

## What Changes

- **VectorStore interface** — выделить интерфейс, `rag/engine.go` зависит от интерфейса, а не от `store.Store`
- **Search algorithms → rag/** — перенести `RRF()`, `KeywordOverlapSearch()` и `TopK()` из `store/qdrant.go` в `internal/rag/`
- **Domain types** — разбить `model/types.go` на логические файлы по доменам: extract, llm, rag, pipeline, events
- **HTTP server** — реализовать `internal/api/` с роутером и хендлерами, `cmd/media2rag/serve.go` — тонкая обёртка
- **Status check Qdrant** — `cmd/media2rag/status.go` проверяет Ollama и rdrr, но не Qdrant

## Capabilities

### New Capabilities

- `vector-store-interface`: Абстракция VectorStore с единым интерфейсом, имплементация Qdrant за интерфейсом
- `search-ranking`: Алгоритмы ранжирования (RRF, KeywordOverlap, TopK) — отдельный пакет или внутри rag, не в store
- `domain-types`: Доменные типы, разделённые по bounded context, без циклических зависимостей
- `http-server`: HTTP API сервер с роутером, хендлерами, graceful shutdown

### Modified Capabilities

<!-- No existing specs to modify -->

## Impact

| Компонент | Изменение |
|-----------|-----------|
| `internal/rag/engine.go` | Зависимость от `VectorStore` interface вместо `store.Store` |
| `internal/store/qdrant.go` | Удалить RRF, KeywordOverlapSearch, TopK |
| `internal/rag/` | Добавить ranking.go с RRF, KeywordOverlapSearch, TopK |
| `internal/model/types.go` | Разбить на >1 файл, возможно >1 пакет |
| `internal/store/` | Переименовать qdrant.go или имплементировать VectorStore |
| `internal/api/` | Новый пакет (роутер + хендлеры) |
| `cmd/media2rag/serve.go` | Тонкая обёртка, вызывает internal/api |
| `cmd/media2rag/status.go` | Добавить проверку Qdrant |

Нет новых внешних зависимостей. Go 1.22+.
