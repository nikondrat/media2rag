## Context

Текущий пайплайн CTG (Compressor → Transformer → Generator) оптимизирован под один тип контента — разговорные транскрипты. Compressor использует LLM prompt "Clean up this raw transcript" с инструкциями удалять таймкоды и filler words. Для PDF/EPUB это разрушает структурный контент: списки фаз, определения, схемы.

Архитектура:
- `cli.py` → выбирает extractor по расширению → `CTGPipeline.process()`
- `CTGPipeline` → для больших документов (>50K chars) использует `ChunkedTransformer.map_reduce()`, для маленьких — `Compressor` + `Transformer`
- `SemanticChunker` делит по заголовкам с overlap 800 chars
- `Transformer` форсирует JSON-структуру с секциями Thesis/Mechanism/Pattern/Framework/Definitions/Quotes/Evidence
- Изображения извлекаются в `PdfEpubExtractor` но не передаются в пайплайн обработки

Проблема: нет различения типов контента на уровне пайплайна. Всё проходит через один prompt.

## Goals / Non-Goals

**Goals:**
- PDF/EPUB структурный контент сохраняется без пересказа LLM
- Диаграммы анализируются через vision LLM, описание встраивается в markdown
- Чанки не разрывают связанные секции (Phase A-E, главы)
- Транскрипты продолжают обрабатываться как раньше (без регрессии)
- Всё работает с Ollama (gemma3:26b vision) и OpenRouter

**Non-Goals:**
- Не переписывать весь пайплайн с нуля — эволюционные изменения
- Не добавлять OCR для изображений (уже есть pytesseract fallback)
- Не менять формат выходного markdown (frontmatter + sections)
- Не поддерживать все форматы изображений — только те, что извлекаются из PDF/EPUB

## Decisions

### D1: Content-type detection на уровне extractor, не LLM

**Решение:** `ExtractedContent` получает поле `content_type: str` со значениями `transcript | structured-document | mixed`. Определяется в extractor по эвристикам, не через LLM.

**Эвристики для structured-document:**
- Есть заголовки `#` / `##` / `###` (≥3 штук)
- Есть нумерованные или маркированные списки (≥5 элементов)
- Есть таблицы или структурированные блоки
- Нет таймкодов `[0:00]`, `[1:30]`
- Отношение заголовков к тексту > 5%

**Альтернатива:** LLM-классификация первого чанка. Отклонена — лишняя задержка, unreliable для локальных моделей.

### D2: Compressor — два режима, один класс

**Решение:** `Compressor` получает метод `compress_structured()` параллельно с существующим `compress()`. Не отдельный класс, чтобы не дублировать chunking логику.

**`compress_structured()` поведение:**
- Не отправляет текст в LLM для пересказа
- Только regex-очистка: OCR-артефакты (разорванные слова, лишние переносы), дублирующиеся заголовки, пустые строки
- Сохраняет иерархию заголовков и списков intact
- Для чанков > max_input_tokens: делит по границам заголовков, не по параграфам

**Альтернатива:** Отдельный класс `StructurePreservingCompressor`. Отклонена — дублирование chunking/splitting логики.

### D3: Image analysis — отдельный модуль, вызывается в extractor

**Решение:** Новый модуль `processors/image_analyzer.py` с классом `ImageAnalyzer`. Вызывается из `PdfEpubExtractor.extract()` ПОСЛЕ извлечения текста, ДО передачи в пайплайн.

**Поток:**
1. Extractor извлекает изображения → сохраняет в `_images/`
2. Для каждого изображения: определяет тип (diagram/chart/photo/text)
3. Для diagram/chart: отправляет в vision LLM с prompt "Describe this diagram in detail for a text-only reader"
4. Описание вставляется в raw_text как markdown-комментарий рядом с `![image]()` референсом
5. `ExtractedContent.images` получает поле `description`

**Фильтрация изображений:**
- Пропускаем изображения < 5KB (иконки, декор)
- Пропускаем изображения с aspect ratio > 10:1 или < 1:10 (линии, разделители)
- Анализируем только изображения 5KB-5MB с разумным aspect ratio

**Альтернатива:** Анализ в Generator после трансформации. Отклонена — Generator не должен делать LLM-вызовы, он только собирает frontmatter.

### D4: Hierarchy-preserving chunking — модификация SemanticChunker

**Решение:** `SemanticChunker` получает параметр `preserve_sections: bool`. Когда True:

- Границы чанков = только границы заголовков уровня H1/H2 (# и ##)
- Не допускаем разрыв внутри секции: если секция > TARGET_SIZE, она становится отдельным чанком (превышение размера допустимо)
- Overlap добавляется только между секциями, не внутри
- Связанные секции (Phase A, Phase B, Phase C...) группируются если помещаются в TARGET_SIZE

**Связанные секции:** детектятся по паттерну — заголовки с общим префиксом ("Фаза", "Phase", "Глава", "Chapter") + нумерация.

**Альтернатива:** Отдельный класс `HierarchicalChunker`. Отклонена — 90% логики совпадает с SemanticChunker.

### D5: Content routing — новый слой в CTGPipeline

**Решение:** `CTGPipeline.process()` получает шаг routing после extraction:

```
extracted → detect_content_type() → route:
  transcript:    Compressor.compress() → Transformer.transform() → Generator
  structured:    Compressor.compress_structured() → Transformer.transform_structured() → Generator
  mixed:         Split by content type → process each → merge
```

`Transformer.transform_structured()` — упрощённый prompt, который не форсирует Thesis/Mechanism/Pattern, а сохраняет исходную структуру документа, только чистит шум.

**Альтернатива:** Отдельный пайплайн для каждого типа. Отклонена — дублирование слишком велико.

### D6: Transformer — два prompt'а, один класс

**Решение:** `Transformer` получает `SYSTEM_PROMPT_STRUCTURED` параллельно с существующим `SYSTEM_PROMPT`.

**`SYSTEM_PROMPT_STRUCTURED`:**
- Не требует JSON-ответ с Thesis/Mechanism/Pattern
- Сохраняет исходные заголовки документа
- Только: удаляет CTA, greetings, sign-offs, self-promotion
- Извлекает metadata (title, author, domains, key_terms) в JSON
- Body = исходный текст с минимальной очисткой

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Vision LLM для изображений — дорого/медленно на Ollama | Фильтрация по размеру/типу, только diagram/chart, не все изображения |
| `compress_structured()` может пропустить реальный шум в PDF | Regex-очистка покрывает 95% OCR-артефактов; для edge cases — `--extract-only` режим |
| Hierarchy-preserving chunking создаёт неравномерные чанки | Допустимо — лучше неравномерные чанки, чем разорванные секции |
| Content-type detection может ошибаться | Fallback на transcript mode если不确定; mixed mode для гибридов |
| Обратная совместимость — старые транскрипты должны работать | transcript mode unchanged; routing только для новых типов |
| OpenRouter vision модели могут не поддерживать все форматы | Fallback: если vision недоступен, вставляем placeholder с alt-text из filename |

## Migration Plan

1. Добавить `content_type` в `ExtractedContent` — backwards compatible (default `transcript`)
2. Добавить `compress_structured()` — не влияет на существующий flow
3. Добавить `ImageAnalyzer` — opt-in, вызывается только если изображения есть
4. Добавить routing в `CTGPipeline` — по умолчанию transcript mode
5. Добавить `transform_structured()` — не влияет на существующий flow
6. Включить routing по умолчанию для PDF/EPUB

Rollback: убрать routing в `CTGPipeline.process()`, всё вернётся к старому поведению.

## Open Questions

1. **Vision model для Ollama:** gemma3:26b поддерживает vision, но какой prompt даёт лучшее описание диаграмм? Нужен эмпирический подбор.
2. **Mixed content:** как определять границу между transcript-частью и structured-частью в одном документе? Эвристика: если >30% текста — списки/заголовки, считаем mixed.
3. **Image description length:** ограничивать ли длину описания диаграммы? Слишком длинное — засоряет output, слишком короткое — теряет информацию.
