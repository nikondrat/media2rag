# Strategy — media2rag

## Current Status (2026-06-10)

| Область | Статус |
|---------|--------|
| Pipeline (CTG) | ✅ Стабильно |
| URL processing (rdrr) | ✅ Работает |
| Batch с параллельными файлами | ✅ Работает |
| Retry logic | ✅ Pipeline + OpenRouter |
| Progress bar + ETA | ✅ Работает |
| Health check | ✅ Работает |
| Telemetry + Pricing | ✅ Работает |
| LLM parsing (robust) | ⚠️ Улучшено, multi-line поля ещё хрупкие |
| Batch statistics | ⏳ Только базовые (processed/skipped/failed) |
| Qdrant vector search | ✅ RAG команда реализована |
| New formats (PDF, audio...) | ❌ Пока .md + URL |

## Phase 1: CLI стабилизация (Now)

**Цель:** Надёжный pipeline обработки любых форматов.

### Статус выполнения
- [x] Параллельная обработка файлов (file_concurrency + total_concurrency)
- [ ] Новые форматы: PDF, audio, video → md → process (пока только .md + URL)
- [x] Retry logic с exponential backoff (pipeline + OpenRouter)
- [x] Robust LLM parsing (улучшено: валидация + template detection + retry)
- [x] Progress bar + ETA (mpv, per-file ETA)
- [ ] Batch statistics (базовые есть, полные — нет)

**Метрика:** 200+ файлов обрабатываются стабильно, любые форматы.

---

## Phase 2: RAG + Qdrant (CLI команды) ✅ COMPLETE

**Цель:** Векторный поиск по знаниям.

### Реализовано
- [x] Qdrant client (HTTP API)
- [x] Команда `media2rag index` — индексация chunks в Qdrant
- [x] Команда `media2rag rag <query>` — hybrid search (dense + sparse + RRF)
- [x] JSON output для AI агентов

**Метрика:** Запрос "какой бизнес в логистике" возвращает релевантные chunks.

> GraphRAG вынесен в отдельную ветку `experimental/graphrag`.

---

## Phase 3: AI Agent Integration

**Цель:** AI агенты используют media2rag как tool.

### Сценарий
```bash
# Hermes (или другой агент) вызывает CLI
hermes> Проанализируй рынок складской недвижимости

# Агент вызывает media2rag
media2rag rag "рынок складов" --format json

# Получает релевантные chunks с метаданными
# Строит отчёт на основе поиска
```

**Метрика:** AI агент самостоятельно проводит анализ, используя RAG.

---

## Future: Memory (возможно)

**Идея:** Знания о пользователе тоже в графе.

```
User Profile Graph:
  [skills] → Go, Python, LLM pipelines
  [gaps] → регуляторика пищевых продуктов
  [strengths] → автоматизация, ETL
  [weaknesses] → over-engineering
```

При запросе RAG подбирает релевантные знания о пользователе и применяет их.

**Решение:** Обсудить отдельно, нужно ли.

---

## Why This Order

1. **Стабильность** → без надёжного pipeline ничего не работает
2. **RAG** → ядро системы, векторный поиск
3. **AI агенты** → используют готовые CLI команды как tools
4. **Memory** → опционально, если будет запрос

Не наоборот. Не "сначала MCP server".

---

## Competitive Context

| | AnythingLLM | ChatGPT files | OpenWebUI | media2rag |
|---|---|---|---|---|
| Подготовка данных | Нет | Нет | Нет | CTG Pipeline |
| Качество ответа | Среднее | Среднее | Среднее | Высокое |
| Knowledge Graph | Ветка experimental | Нет | Нет | Ветка experimental |
| Causal chains | Нет | Нет | Нет | ✅ |
| AI agent integration | Нет | Нет | Ограничено | CLI tool |
| Стабильность | Падает на 200 файлах | Облако | Docker | Single binary |
| Локальность | Docker | Нет | Docker | Да |

**Наше отличие:** данные подготовлены + CTG Pipeline → AI отвечает с контекстом, а не просто похожими текстами.
