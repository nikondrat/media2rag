## Why

После Extraction (Фаза 2) `process` извлекает сырой Markdown и сохраняет в workspace. Но для RAG нужен структурированный, очищенный документ с метаданными. CTG Pipeline (Compress → Transform → Generate) превращает сырой текст в RAG-ready Markdown: убирает мусор, разбивает на чанки, извлекает темы/саммари, собирает финальный документ с YAML frontmatter.

## What Changes

- `internal/pipeline/` — пакет CTG: Compressor, Splitter, Processor, Assembler
- Compressor: LLM-чистка текста с разбивкой на части для больших документов
- Splitter: recursive character split (без LLM)
- Processor: LLM-обработка каждого чанка (title + topics + summary), параллельно
- Assembler: сборка финального Markdown с frontmatter
- Интеграция в `process` команду: extract → pipeline → save version
- Новые события: compression_start, cleaning_part, split_done, processing_chunk, completed

## Capabilities

### New Capabilities
- `ctg-pipeline`: Compress → Split → Process → Assemble pipeline
- `compressor`: LLM-based text cleaning, chunking for large texts
- `splitter`: recursive character split with configurable size/overlap
- `chunk-processor`: per-chunk LLM extraction (title, topics, summary)
- `assembler`: merge chunks into RAGDocument with YAML frontmatter

### Modified Capabilities
- `process-command`: теперь запускает pipeline после extraction
- `workspace`: добавляется SaveVersion для pipeline output

## Impact

- LLM вызовы: Compressor (1+ вызовов) + Processor (N вызовов, где N = число чанков)
- Параллельная обработка чанков через worker pool (configurable concurrency)
- Время обработки увеличивается, но качество документа значительно выше
- Не создаётся: RAG indexing (Qdrant) — это фаза 4
