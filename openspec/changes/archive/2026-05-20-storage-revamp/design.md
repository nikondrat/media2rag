## Context

Сейчас артефакты обработки хранятся разрозненно:
- `.chunks/<stem>/chunk_NNN.md` — промежуточные чанки (в корне repo!)
- `output/<title>.md` — финальный результат
- `output/<title>_raw.md` — промежуточный raw extraction
- Изображения нигде не сохраняются централизованно

`ChunkedTransformer` уже сохраняет merged sections (`merged_<section>.md`) в chunk_dir, но они смешаны с chunk файлами.

## Goals / Non-Goals

**Goals:**
- Единая структура workspace: все артефакты файла в одной папке
- CLI сохраняет секции отдельно
- Workspace path настраивается через `--workspace` / env var
- Обратная совместимость не требуется — это новый CLI

**Non-Goals:**
- Миграция старых `.chunks/` и `output/` директорий
- Изменение формата чанков или секций
- Изменение CTG pipeline логики

## Decisions

### 1. Workspace structure

```
<workspace>/
└── <file-stem>/
    ├── chunks/          # Raw chunk outputs (chunk_001.md, metadata.json)
    ├── images/          # Extracted images (img_001.png, ...)
    ├── sections/        # Merged sections (thesis.md, mechanism.md, ...)
    ├── intermediate/    # Raw extraction (raw.md)
    └── output/          # Final RAG document (final.md)
```

`<file-stem>` — sanitized filename или URL host+path. Используется `_sanitize_filename()` из `domain/document.py`.

### 2. Workspace path resolution

Priority: `--workspace` CLI arg > `WORKSPACE` env var > `output_dir` from settings > `~/Documents/media2rag/`

CLI получает `workspace_dir` вместо `output_dir`. Старый `-o` флаг переименован в `--workspace` (alias для обратной совместимости).

### 3. Section output

В `ChunkedTransformer._reduce()`: после `_merge_sections_with_threshold()` каждая merged section записывается в `sections/<name>.md` вместо chunk_dir. Финальный документ собирается из sections/ как раньше.

### 4. Image extraction

`PdfEpubExtractor` при извлечении изображений из EPUB/PDF сохраняет их в `images/` workspace. В `ExtractedContent` добавляется `image_paths: list[Path]`.

### 5. JSON events

Добавлены поля:
- `work_dir`: путь к workspace папке файла
- `sections`: список имён сохранённых секций
- `images`: список путей к изображениям

### 6. RAGDocument.save()

Метод принимает `workspace_dir` вместо `output_dir`. Пишет `final.md` в `output/`, `raw.md` в `intermediate/`.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Breaking change для скриптов, парсящих output/ | Документировать в README, `--workspace` alias для `-o` |
| Большие workspace (тысячи файлов) | Каждая папка изолирована, можно удалять целиком |
| Конфликты имён (одинаковый stem) | `_sanitize_filename()` + UUID suffix при конфликте |
| EPUB image extraction замедляет процесс | Опционально, флаг `--extract-images` |
