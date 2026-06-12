## Context

LLM-клиенты (Фаза 1) уже реализованы с `Chat`, `StreamChat`, `Embed`. Нужно добавить единый формат выводов и парсер.

**Constraints:**
- Формат: `> type(params)\ncontent\n<`
- Только Markdown, никаких JSON/XML в LLM output
- Один парсер для всех случаев
- Fallback: если LLM не следует формату → plain text block

## Goals / Non-Goals

**Goals:**
- `ParseOutput(text)` парсит `> type\ncontent\n<` → `[]TypedBlock`
- `ChatAndParse` — вызывает LLM, парсит ответ
- `StreamAndParse` — стримит токены, парсит финальный результат
- Промпты используют Markdown-only формат

**Non-Goals:**
- Обновление промптов в pipeline/RAG/chat — это делается в соответствующих фазах
- Валидация формата на стороне LLM — только парсинг + fallback

## Decisions

### 1. Формат: `> type(params)\ncontent\n<`
**Why:** `>` и `<` — естественные Markdown маркеры (blockquote). LLM хорошо их понимает. `params` через `:` и `,` — просто парсить.
**Alternatives considered:** `[[type]]` — не Markdown, LLM путает. XML-теги — LLM ломает вложенность.

### 2. Params: `key=value` через `, `
**Why:** Просто парсить: `strings.SplitN` по `:`, потом `strings.Split` по `, `.
**Format:** `> memory: user=nikita, category=fact\ncontent\n<`

### 3. Fallback на plain text
**Why:** LLM может не следовать формату. Если нет `> ` → весь ответ = один block type="text".
**Alternatives considered:** Error — ломает pipeline, retry — лишний вызов.

### 4. Одна функция `ParseOutput`
**Why:** Все пакеты (pipeline, rag, chat, memory) используют один парсер. Нет дублирования логики.

### 5. `ChatAndParse` wrapper
**Why:** Удобный метод: `client.ChatAndParse(ctx, prompt)` → `[]TypedBlock`. Не нужно вызывать Chat + Parse отдельно.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| LLM путает `>` в контенте с маркером | Парсер ищет `> type\n` с newline, не просто `>` |
| LLM забывает `<` | Парсер закрывает block по следующему `>` или EOF |
| Params с special chars | URL-encode или quoting если нужно |
