## Context

Проект media2rag v2 — переписывание с Python на Go. Текущий репозиторий содержит только документацию (`docs/`), Go-кода нет. Нужен фундамент: модуль, структура, конфигурация, модели, LLM-клиенты, event emitter, CLI-скелет.

**Constraints:**
- Pure Go, no CGo (для фазы 1)
- Zero runtime dependencies кроме Ollama (localhost:11434)
- Конфигурация: `.env` → `~/.media2rag/config.yaml` → CLI flags (приоритет)
- Все операции эмитят JSON-события в stdout (newline-delimited)

## Goals / Non-Goals

**Goals:**
- `go build ./cmd/media2rag` собирает бинарник
- Бинарник запускается с `--help`, показывает subcommands
- `media2rag process ./file.md` парсит аргументы, загружает конфиг, эмитит события
- LLM-клиент делает chat и embed запросы к Ollama
- Конфигурация загружается из 3 источников с правильным приоритетом

**Non-Goals:**
- Extraction логика (rdrr, PDF, EPUB) — фаза 2
- CTG pipeline — фаза 3
- RAG engine, Qdrant — фаза 4
- Chat sessions, SQLite, memory — фаза 5
- HTTP server, serve mode — фаза 6
- Tests, observability — фаза 7

## Decisions

### 1. CLI framework: `cobra` + `viper`
**Why:** Стандарт де-факто для Go CLI. Cobra для subcommands, viper для конфигурации (env + yaml + flags).
**Alternatives considered:** `urfave/cli` — проще, но viper даёт из коробки multi-source config.

### 2. HTTP client: стандартный `net/http`
**Why:** Для Ollama и OpenRouter достаточно стандартного HTTP клиента. No need for `resty` или аналоги.
**Alternatives considered:** `go-resty` — удобней, но лишняя зависимость для 2 клиентов.

### 3. Config: viper для yaml + `.env`, но CLI flags — вручную через cobra
**Why:** Viper не умеет приоритет CLI flags > env > yaml из коробки без bind. Ручной merge проще и прозрачней.
**Alternatives considered:** Полностью viper с `BindPFlag` — работает, но магия с приоритетами сложно дебажить.

### 4. Event emitter: интерфейс + stdout implementation
**Why:** Serve mode позже будет эмитить события в WebSocket. Интерфейс позволяет переиспользовать логику.
**Design:** `EventEmitter` interface с `Emit(Event)` и `Done()`. `StdoutEmitter` пишет JSON-lines.

### 5. LLM client: интерфейс + 2 реализации
**Why:** Ollama и OpenRouter имеют разные auth, но одинаковый API (OpenAI-compatible). Единый интерфейс `LLMClient` с `Chat`, `StreamChat`, `Embed`.
**Fallback:** Если Ollama недоступен — fallback на OpenRouter (конфигурируемый).

### 6. Project structure:严格按 `docs/architecture.md`
**Why:** Документация уже описала структуру. Следуем ей.
```
cmd/media2rag/main.go
internal/config/config.go
internal/model/types.go
internal/events/emitter.go
internal/llm/client.go, ollama.go, openrouter.go
```

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Viper + cobra bind complexity | Ручной merge config: yaml → env → flags, явно и тестируемо |
| Ollama API changes | Интерфейс `LLMClient` абстрагирует, легко заменить реализацию |
| Event format inconsistency | Единый `Event` struct в `model`, все эмиттеры используют его |
| Over-engineering foundation | Фаза 1 — только то, что нужно для `process` с local markdown |

## Migration Plan

Не применяется — новая кодовая база, Python не затрагивается.

## Open Questions

- Embedding model: какой модель использовать для embed? (по умолчанию `nomic-embed-text` в Ollama)
- Timeout для LLM-запросов: дефолт 30s или конфигурируемый?
