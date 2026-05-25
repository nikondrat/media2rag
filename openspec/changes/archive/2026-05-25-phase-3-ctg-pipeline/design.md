## Context

После Фазы 2 есть extraction и workspace. Сырой Markdown извлекается и сохраняется. Фаза 3 добавляет CTG Pipeline — превращение сырого текста в RAG-ready документ.

**Constraints:**
- Каждый этап — отдельная функция, не монолит
- Параллельная обработка чанков через worker pool
- Все этапы эмитят события через EventEmitter
- Конфигурируемые размеры чанков, concurrency

## Goals / Non-Goals

**Goals:**
- Compressor: LLM-чистка текста, разбивка на части для больших документов
- Splitter: recursive character split (без LLM)
- Processor: per-chunk LLM (title + topics + summary), параллельно
- Assembler: финальный Markdown с YAML frontmatter
- `process` команда запускает pipeline после extraction

**Non-Goals:**
- Claims, mental models, key terms extraction — v2+
- Quality check LLM — отложено
- RAG indexing — фаза 4

## Decisions

### 1. Pipeline как цепочка Stage функций
**Why:** `type Stage func(ctx, input, emitter) (string, error)` — простая композиция, лёгко тестировать, добавлять этапы.
**Alternatives considered:** Struct-based pipeline с методами — больше boilerplate, менее гибко.

### 2. Splitter без LLM
**Why:** Recursive character split — детерминированный, быстрый, не тратит LLM-вызовы. Разделители: `\n\n\n` → `\n\n` → `\n` → `. ` → hard cut.
**Alternatives considered:** LLM-based splitting — дорого, медленно, непредсказуемо.

### 3. Worker pool для чанков
**Why:** LLM вызовы I/O-bound, goroutines идеальны. Configurable concurrency (дефолт: 3).
**Alternatives considered:** Последовательная обработка — в 3x медленнее для типичного документа.

### 4. Process chunk: один промпт для title+topics+summary
**Why:** Маленький фокусный промпт → предсказуемый KV-ответ → лёгкий парсинг. LLM не "халтурит" на конкретной задаче.
**Alternatives considered:** Отдельные промпты для каждого поля — 3x вызовов на чанк, дорого.

### 5. Assembler: Go-native, без LLM
**Why:** Merge topics (dedup + frequency), pick title (first non-empty), build summary (concat) — всё детерминировано. LLM не нужен.
**Alternatives considered:** LLM merge — дорого, непредсказуемо.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| LLM rate limiting при параллельных вызовах | Worker pool с configurable concurrency |
| Большие документы (>100K chars) | Compressor разбивает на части, чистит каждую |
| Нестабильный KV-парсинг | Regex fallback, validation |
