## Context

После Фазы 1 есть Foundation: Go-модуль, config, LLM clients, event emitter, CLI skeleton. Фаза 2 добавляет extraction и workspace — первый end-to-end сценарий `process`.

**Constraints:**
- Только 2 экстрактора: URL (rdrr) + LocalFile (.md)
- PDF, EPUB, audio — "unsupported format" с понятной ошибкой
- Workspace: filesystem-based, hash-based директории
- Версионирование: symlink `latest`, папки `versions/vN/`

## Goals / Non-Goals

**Goals:**
- `media2rag process ./notes.md` → source.md в workspace
- `media2rag process "https://youtube.com/..."` → rdrr → source.md в workspace
- `media2rag documents list` → список документов
- `media2rag status` → health check
- Все операции эмитят JSON-события с `--json`

**Non-Goals:**
- CTG pipeline (Compress/Transform/Generate) — фаза 3
- RAG indexing в Qdrant — фаза 4
- PDF, EPUB, audio extraction — фаза 5
- HTTP serve mode — фаза 6

## Decisions

### 1. Extractor возвращает string (Markdown), не ExtractedContent
**Why:** Metadata извлекается позже, на этапе Compress. Extractor — только сырой Markdown. Это упрощает интерфейс и разделяет ответственность.
**Alternatives considered:** Возвращать `ExtractedContent` — но тогда extractor должен парсить metadata, что дублирует логику pipeline.

### 2. rdrr через `exec.CommandContext`, не HTTP
**Why:** rdrr — CLI-утилита. Запускать как subprocess проще, чем поднимать HTTP-сервис. `context.Context` даёт timeout и cancellation.
**Alternatives considered:** Временный HTTP-сервер для локальных файлов — отложено до фазы 5 (PDF).

### 3. Workspace: filesystem, не SQLite
**Why:** Documents — это файлы. SQLite нужен для sessions/memory (фаза 5). Workspace — просто FS с metadata YAML.
**Alternatives considered:** SQLite для tracking документов — лишняя сложность, FS достаточно.

### 4. Source-hash: первые 8 символов SHA-256
**Why:** Уникально достаточно для personal use, коротко для readability. Полный SHA-256 — 64 символа, неудобно.
**Collision risk:** 8 hex chars = 2^32 space. Для <1000 документов вероятность negligible.

### 5. Версионирование: папки + symlink
**Why:** Простая схема. `versions/vN/final.md` + symlink `latest → vN`. Легко навигировать вручную.
**Alternatives considered:** Git-подобная схема — overkill для v1.

### 6. Process без pipeline = extract-only
**Why:** CTG pipeline ещё не реализован. `process` пока делает только extract → save source.md. Это полезно само по себе: пользователь получает чистый Markdown из URL.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| rdrr не установлен | Чёткая ошибка: "rdrr not found. Install: npx rdrr" |
| rdrr медленный на больших URL | Timeout через context, event "extracting" с прогрессом |
| Hash collision в workspace | Увеличить до 16 символов если проблема |
| Symlink на Windows | fallback: `latest.txt` с путём вместо symlink |

## Migration Plan

Не применяется — новая кодовая база.

## Open Questions

- rdrr fallback: если `--json` не сработал, парсить plain text output?
- Workspace location: configurable через `--workspace` или только config?
