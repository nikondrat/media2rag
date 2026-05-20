## Why

Файлы результата обработки разбросаны по разным местам: чанки в `.chunks/<stem>/`, финальный вывод в `output/`, промежуточные файлы рядом с результатом. Нет единой структуры на файл — невозможно увидеть все артефакты обработки, сложно очищать старые данные, нет поддержки изображений и секций.

## What Changes

- Введена единая структура workspace: `<workspace>/<file-stem>/` с подпапками `chunks/`, `images/`, `sections/`, `output/`, `intermediate/`
- CLI сохраняет каждую merged-секцию отдельно в `sections/<name>.md` вместо только финального файла
- Изображения (из EPUB, video thumbnails) сохраняются в `images/`
- Workspace path настраивается (дефолт `~/Documents/media2rag/`)
- JSON events обновлены: добавлены `work_dir`, `sections`, `images` поля
- Старые пути (`.chunks/`, плоский `output/`) больше не используются — **BREAKING** для скриптов, зависящих от старой структуры

## Capabilities

### New Capabilities
- `workspace-structure`: Единая иерархия хранения артефактов на файл
- `section-output`: CLI сохраняет секции отдельно в `sections/` папку
- `image-storage`: Изображения извлекаются и сохраняются в `images/`

### Modified Capabilities
- `cli-json-events`: Добавлены новые поля в JSON events (`work_dir`, `sections`, `images`)

## Impact

- `cli.py` — изменение путей сохранения, новая логика workspace
- `processors/chunked_transformer.py` — сохранение merged sections в `sections/`
- `domain/document.py` — RAGDocument.save() пишет в workspace структуру
- `extractors/pdf_epub_extractor.py` — извлечение изображений в `images/`
- `media2rag-gui` — QueueManager, SettingsManager обновлены для workspace path
- Существующие `.chunks/` директории не мигрируются автоматически
