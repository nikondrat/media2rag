## Why

CTG pipeline был спроектирован под транскрипты (YouTube, подкасты) — удаление таймкодов, filler words, CTA. При обработке PDF/EPUB книг это ломает контент: Compressor пересказывает структурные схемы (фазы A-E Вайкоффа) как prose, уничтожая списки определений. Изображения (диаграммы) извлекаются как `![image](path)` но игнорируются пайплайном. Chunker рвёт связанные секции на границах. Transformer форсирует единый шаблон Thesis/Mechanism/Pattern для всего, включая контент, который не вписывается.

Результат: галлюцинации в 30%+ PDF-контента, потеря диаграмм, обрывки текста на границах чанков.

## What Changes

- **Compressor** получает режим `preserve-structure` для PDF/EPUB — распознаёт иерархию заголовков и списков, не пересказывает, а чистит артефакты OCR
- **Image analysis** — диаграммы анализируются через vision LLM, описание встраивается в markdown рядом с местом изображения
- **Chunker** получает режим `preserve-hierarchy` — чанки не рвутся внутри связанных секций (Phase A-E), граница чанка = граница секции
- **Content-type routing** — пайплайн определяет тип контента (transcript vs structured-document) и применяет разные стратегии обработки

## Capabilities

### New Capabilities
- `structure-aware-compression`: Compressor распознаёт структурный контент (списки, фазы, определения) и сохраняет его целостно, не пересказывая
- `image-content-analysis`: Извлечённые изображения анализируются через vision LLM, описание встраивается в выходной markdown
- `hierarchy-preserving-chunking`: Chunker делит текст по семантическим границам секций, не разрывая связанные блоки (Phase A-E)
- `content-type-routing`: Пайплайн определяет тип входного контента и маршрутизирует через разные стратегии обработки

### Modified Capabilities

## Impact

- `processors/compressor.py` — новый режим для PDF/EPUB
- `processors/chunker.py` — hierarchy-preserving mode
- `processors/chunked_transformer.py` — routing logic
- `processors/ctg_pipeline.py` — content-type detection + routing
- `extractors/pdf_epub_extractor.py` — передача image metadata в пайплайн
- `domain/document.py` — ImageDescription модель
- Новый модуль `processors/image_analyzer.py`
- Новый модуль `processors/content_router.py`
