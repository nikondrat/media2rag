## Why

После Foundation (Фаза 1) у нас есть Go-модуль, конфигурация, LLM-клиенты и CLI-скелет. Следующий шаг — заставить `process` команду реально работать: извлечь контент из URL или локального Markdown, сохранить в workspace, вывести результат. Это первый end-to-end сценарий: пользователь даёт URL/файл → получает RAG-ready Markdown.

## What Changes

- `internal/extract/` — пакет экстракторов: URL (через `npx rdrr`) + LocalFile (Go-native для `.md`)
- `internal/workspace/` — пакет workspace: hash-based директории, версионирование, metadata
- `process` команда — полная реализация: detect → extract → save → emit events
- `documents` команда — subcommands: `list`, `show`, `delete`
- `status` команда — проверка доступности Ollama, rdrr, workspace

## Capabilities

### New Capabilities
- `extractors`: Extractor interface, Registry, URLExtractor (rdrr), LocalFileExtractor (.md)
- `workspace`: source-hash dirs, versioning, metadata YAML, CRUD operations
- `process-command`: полный цикл process с event streaming
- `documents-command`: list, show, delete документов из workspace
- `status-command`: health check внешних зависимостей

### Modified Capabilities
- `cli-skeleton`: `process` команда получает полную реализацию вместо заглушки

## Impact

- Runtime-зависимость: `npx rdrr` должен быть доступен для URL extraction
- Workspace создаётся в `~/.media2rag/workspace`
- `process` команда теперь полноценная, не заглушка
- Не создаётся: CTG pipeline (Compress/Transform/Generate) — это фаза 3
- Не создаётся: PDF, EPUB, audio экстракторы — это фаза 5
