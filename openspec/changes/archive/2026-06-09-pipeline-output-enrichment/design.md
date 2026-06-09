## Context

CTG Pipeline v1 (реализован) — Compress → Split → Process (title+topics+summary) → Assemble → базовый frontmatter. Python версия выдаёт документ с core_thesis, claims, mental_models, takeaways, key_terms, domains. Разрыв из-за двух проблем:

1. **Process stage — только v1 промпт.** План расширения (docs/ctg-pipeline.md v2-v6) не реализован
2. **Pipeline не видит ExtractedContent.** Source, type, author, language теряются

**Constraints:**
- Каждый промпт — маленький, focused. Не монолит
- v2-v6 промпты — параллельные LLM вызовы на каждый чанк (worker pool)
- Holistic pass — отдельный LLM вызов после всех чанков
- Pipeline API не ломать обратную совместимость

## Goals / Non-Goals

**Goals:**
- Выходной .md файл соответствует Python версии по богатству
- v2-v6 промпты: claims, mental_models, key_terms, core_thesis, takeaways
- Holistic pass для cross-chunk полей (core_thesis, domains)
- Frontmatter со всеми полями DocumentMetadata
- Pipeline принимает ExtractedContent (source, type, author, language)
- Output format spec — зафиксировать структуру

**Non-Goals:**
- Quality check LLM — отложено
- RAG indexing — фаза 4
- Изменение Compressor/Splitter — работают как есть

## Decisions

### 1. Multi-prompt per chunk (параллельные вызовы)
**Why:** Каждый промпт — маленький, focused. 6 промптов × 1 LLM вызов = 6 параллельных вызовов на чанк. Worker pool (concurrency=3) управляет нагрузкой.
**How:**
```
Chunk N — параллельно:
  ├── prompt v1: title + topics + summary
  ├── prompt v2: extract claims
  ├── prompt v3: extract mental models
  ├── prompt v4: extract key terms
  ├── prompt v5: extract core thesis
  └── prompt v6: extract takeaways
```
Все результаты мержатся в один ChunkResult.

### 2. Holistic pass — отдельный этап после Process
**Why:** Core thesis, domains видны только при взгляде на весь документ. Per-chunk анализ теряет контекст.
**How:** После Process stage (все чанки обработаны) → LLM вызов на cleaned текст целиком → извлекает core_thesis, domains. Результат мержится в финальный DocumentMetadata с приоритетом над per-chunk значениями.

### 3. Pipeline API: Run(ctx, ExtractedContent, emitter) → RAGDocument
**Why:** Source, type, author, language приходят из extractor'а. Pipeline должен их получить для frontmatter.
**How:** `ExtractedContent` уже есть в model/types.go. Меняем сигнатуру Run (и добавляем старый RunString для совместимости).

### 4. ChunkResult — расширяется
**Why:** Каждый v2-v6 промпт добавляет поля.
```go
type ChunkResult struct {
    Title        string
    Topics       []string
    Summary      string
    Claims       []Claim
    MentalModels []string
    KeyTerms     []KeyTerm
    CoreThesis   string
    Takeaways    []string
    Content      string
    Index        int
}
```

### 5. Assembler — holistic merge
**Why:** После merge per-chunk результатов, holistic pass может перезаписать core_thesis и domains.
**Order:**
1. Merge per-chunk результатов (frequency dedup)
2. Holistic LLM вызов
3. Holistic результат имеет приоритет для core_thesis, domains
4. Генерация YAML frontmatter со всеми полями

### 6. Config — включать/выключать v2-v6
**Why:** Не всем нужны все промпты. Можно включить только нужные уровни.
```go
type PipelineConfig struct {
    // ... существующие поля
    ExtractClaims      bool // v2
    ExtractMentalModels bool // v3
    ExtractKeyTerms    bool // v4
    ExtractCoreThesis  bool // v5
    ExtractTakeaways   bool // v6
}
```

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| 6x LLM вызовов на чанк → дорого | Config-флаги включают только нужные v2-v6 |
| Holistic pass дублирует работу Process | Holistic только для cross-chunk полей (core_thesis, domains) |
| ChunkResult растёт → больше памяти | Только строки/слайсы, без копирования контента |
| Pipeline API change → ломает существующий код | Добавить Run() и сохранить старый RunString() |
