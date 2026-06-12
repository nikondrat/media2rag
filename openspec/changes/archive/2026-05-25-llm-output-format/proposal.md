## Why

Все LLM-вызовы в системе (Compressor, Chunk Processor, Query Rewrite, Reranker, Memory Extraction, Chat) должны возвращать данные в едином парсируемом формате. Без этого каждый промпт генерит свой формат — JSON, XML, plain text — и парсинг становится хрупким. Единый формат `> type(params)\ncontent\n<` позволяет надёжно извлекать structured data из любых LLM-ответов.

## What Changes

- `internal/llm/parse.go` — парсер: `ParseOutput(text) → []TypedBlock`
- `internal/llm/client.go` — `ChatAndParse`, `StreamAndParse` методы
- Обновление всех промптов: вместо JSON/XML → Markdown с `> type\ncontent\n<`
- `TypedBlock` модель: Type, Params map[string]string, Content string

## Capabilities

### New Capabilities
- `output-format-parser`: парсинг `> type(params)\ncontent\n<` → `[]TypedBlock`
- `llm-parse-integration`: `ChatAndParse`, `StreamAndParse` в LLMClient

### Modified Capabilities
- `llm-clients`: добавляются методы с авто-парсингом

## Impact

- Все будущие промпты должны использовать этот формат
- Существующие LLM clients получают новые методы (без breaking changes)
- Промпты в pipeline, RAG, chat будут обновлены в соответствующих фазах
