# Project Context — media2rag (Go)

Go-бинарник, который преобразует любой контент в RAG-ready Markdown. Единый бэкенд для всех клиентов (GUI, web, CLI).

Ключевые компоненты:
- **CLI**: `cmd/media2rag/` — точки входа (process, serve, ask, chat)
- **Core**: `internal/` — экстракция, pipeline, LLM, Qdrant, RAG, чат
- **Config**: `~/.media2rag/config.yaml` + CLI флаги + env

Архитектура и дизайн: `docs/`
