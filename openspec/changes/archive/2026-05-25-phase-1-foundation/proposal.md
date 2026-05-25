## Why

media2rag v2 — переписывание с Python на Go для создания единого бинарника без runtime-зависимостей. Текущая Python-версия — прототип с дублированием логики в CLI/GUI, хрупким CTG-пайплайном и проблемами деплоя. Нужен фундамент: структура проекта, конфигурация, domain-модели, LLM-клиенты и система событий, на которой будут строиться все остальные компоненты.

## What Changes

- Инициализация Go-модуля (`go.mod`) и структуры проекта согласно `docs/architecture.md`
- Система конфигурации: загрузка из `.env`, `~/.media2rag/config.yaml`, CLI-флагов
- Domain-модели (`internal/model/`): `ExtractedContent`, `RAGDocument`, `LLMClient` интерфейсы, `Event`, `MemoryEntry`
- JSON event emitter (`internal/events/`): newline-delimited события в stdout для subprocess mode
- LLM clients (`internal/llm/`): Ollama (localhost:11434) + OpenRouter-compatible клиент с fallback
- Базовые CLI-команды-заглушки: `process`, `serve`, `ask`, `chat` (пока без логики, только parsing flags)

## Capabilities

### New Capabilities
- `config-system`: загрузка и валидация конфигурации из нескольких источников с приоритетом
- `event-emitter`: структурированные JSON-события для прогресса, стриминга, результатов
- `llm-clients`: унифицированный интерфейс к LLM-провайдерам (Ollama, OpenRouter) с chat, stream, embed
- `domain-models`: общие типы данных для extraction, pipeline, RAG, chat, memory
- `cli-skeleton`: CLI-команды с parse flags, help, subcommands

### Modified Capabilities
<!-- None — это новая кодовая база -->

## Impact

- Создаётся новая Go-кодовая база (`cmd/`, `internal/`)
- Python-код остаётся в `main` ветке, не затрагивается
- Внешние зависимости: Ollama (localhost:11434) для LLM-вызовов
- Не создаётся: extraction, pipeline, RAG, chat logic, SQLite, Qdrant — это следующие фазы
