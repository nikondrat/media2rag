## 1. ChunkResult — расширение структуры

- [x] 1.1 Добавить Claims, MentalModels, KeyTerms, CoreThesis, Takeaways, Domains в ChunkResult
- [x] 1.2 Добавить ExtractClaims, ExtractMentalModels, ExtractKeyTerms, ExtractCoreThesis, ExtractTakeaways, HolisticAnalysis в PipelineConfig

## 2. Process stage — v2-v6 промпты

- [x] 2.1 Implement claims extraction prompt + KV parsing
- [x] 2.2 Implement mental models extraction prompt + parsing
- [x] 2.3 Implement key terms extraction prompt + parsing
- [x] 2.4 Implement core thesis extraction prompt + parsing
- [x] 2.5 Implement takeaways extraction prompt + parsing
- [x] 2.6 Сделать все промпты параллельными (worker pool на промпты внутри чанка)
- [x] 2.7 Учесть config-флаги: если false — скипать промпт

## 3. Pipeline API — ExtractedContent

- [x] 3.1 Изменить сигнатуру: `Run(ctx, ExtractedContent, emitter)` → RAGDocument
- [x] 3.2 Пробросить Source, DocType, Author, Language через Pipeline в Assembler
- [x] 3.3 Сохранить старый `RunString(ctx, rawText, emitter)` для совместимости

## 4. Holistic stage

- [x] 4.1 Implement holistic LLM prompt (core_thesis + domains из всего текста)
- [x] 4.2 Интегрировать holistic pass в Pipeline.Run между Process и Assemble
- [x] 4.3 Holistic результат имеет приоритет для core_thesis и domains

## 5. Assembler — обогащённый frontmatter

- [x] 5.1 Merge claims с дедупликацией по тексту
- [x] 5.2 Merge mental_models с дедупликацией
- [x] 5.3 Merge key_terms с дедупликацией по term
- [x] 5.4 Merge takeaways с дедупликацией
- [x] 5.5 Генерация frontmatter со всеми полями (source, type, author, language, domains, core_thesis, mental_models, claims, takeaways, key_terms)
- [x] 5.6 Пропускать пустые поля в frontmatter

## 6. Integration

- [x] 6.1 process.go: передавать ExtractedContent в pipeline (вместо string)
- [x] 6.2 `go build ./cmd/media2rag` succeeds
- [x] 6.3 Проверить output на тестовом файле — сравнить с Python версией
