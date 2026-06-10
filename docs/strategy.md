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
| RAG + GraphRAG команды | 🔧 Causal extraction в pipeline есть, CLI команд нет |
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

## Phase 2: RAG + GraphRAG (CLI команды)

**Цель:** Каузальный поиск по знаниям.

### Задачи
- Восстановить Qdrant из git
- Команда `media2rag rag <query>` — векторный поиск
- Knowledge Graph extraction из chunks (causal_chain, preconditions)
- Команда `media2rag graphrag <query>` — обход графа 2-3 hop
- JSON output для AI агентов (Hermes и др.)

**Метрика:** Запрос "какой бизнес в логистике" возвращает цепочку: проблема → решение → возможность.

---

## Phase 3: AI Agent Integration

**Цель:** AI агенты используют media2rag как tool.

### Сценарий
```bash
# Hermes (или другой агент) вызывает CLI
hermes> Проанализируй рынок складской недвижимости

# Агент вызывает media2rag
media2rag graphrag "рынок складов" --format json

# Получает causal chains + opportunities
# Строит отчёт на основе графа знаний
```

**Метрика:** AI агент самостоятельно проводит анализ, используя GraphRAG.

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

При запросе GraphRAG подбирает релевантные знания о пользователе и применяет их.

**Решение:** Обсудить отдельно, нужно ли.

---

## Why This Order

1. **Стабильность** → без надёжного pipeline ничего не работает
2. **RAG/GraphRAG** → ядро системы, каузальный поиск
3. **AI агенты** → используют готовые CLI команды как tools
4. **Memory** → опционально, если будет запрос

Не наоборот. Не "сначала MCP server".

---

## Competitive Context

| | AnythingLLM | ChatGPT files | OpenWebUI | media2rag |
|---|---|---|---|---|
| Подготовка данных | Нет | Нет | Нет | CTG Pipeline |
| Качество ответа | Среднее | Среднее | Среднее | Высокое |
| GraphRAG | Нет | Нет | Нет | ✅ |
| Causal chains | Нет | Нет | Нет | ✅ |
| AI agent integration | Нет | Нет | Ограничено | CLI tool |
| Стабильность | Падает на 200 файлах | Облако | Docker | Single binary |
| Локальность | Docker | Нет | Docker | Да |

**Наше отличие:** данные подготовлены + каузальный граф → AI отвечает цепочками рассуждений, а не просто похожими текстами.
