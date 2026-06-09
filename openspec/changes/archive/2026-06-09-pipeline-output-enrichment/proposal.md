## Why

CTG Pipeline (фаза 3) делает только базовый per-chunk анализ (title+topics+summary) и не использует metadata из extractor'а. В результате итоговый .md файл содержит минимальный frontmatter и плоский cleaned текст, без глубокой структуры: нет core_thesis, claims, mental_models, takeaways, key_terms, domains. Python версия выдаёт документ в 5x богаче — её формат и берём за эталон.

## What Changes

- **Process stage: добавить v2-v6 промпты** — параллельные LLM вызовы на каждый чанк для claims, mental_models, key_terms, core_thesis, takeaways
- **Pipeline: принимать ExtractedContent** — а не только raw string, чтобы source/type/author/language попадали в frontmatter
- **Assembler: holistic pass** — после merge чанков, дополнительный LLM вызов на весь cleaned текст для cross-chunk полей (core_thesis, domains)
- **Output format spec** — зафиксировать структуру итогового .md файла
- **DocumentMetadata: KeyInsights** — добавить поле (есть в model/types.go, но не заполняется)

## Capabilities

### New Capabilities
- `output-format`: спецификация формата итогового RAG-ready .md файла
- `pipeline-holistic`: holistic LLM pass после Process stage для cross-chunk анализа

### Modified Capabilities
- `chunk-processor`: добавить v2-v6 промпты (claims, mental_models, key_terms, core_thesis, takeaways)
- `assembler`: обогатить frontmatter всеми полями DocumentMetadata
- `ctg-pipeline`: принимать ExtractedContent на вход, а не только string

## Impact

- Process stage: 1 LLM вызов на чанк → до 6 параллельных вызовов на чанк (но можно включать через config)
- Assembler: +1 holistic LLM вызов на весь документ
- Pipeline API: `Run(ctx, ExtractedContent, emitter)` → вместо `Run(ctx, rawText, emitter)`
- process.go: передавать ExtractedContent вместо string
- Compressor, Splitter: без изменений (работают с Content string)
