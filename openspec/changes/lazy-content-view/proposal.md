## Why

`DetailView` загружает весь Markdown файл в память через `String(contentsOf:)`. Для документов 50K+ слов (книги, длинные transcripts) это блокирует UI, жрёт память и делает просмотр невозможным. При map-reduce обработке пользователь не видит статус отдельных чанков и не может взаимодействовать с ними.

## What Changes

- Файлы читаются через `FileHandle.map()` (memory-mapped I/O) — в памяти только видимые секции
- Секции индексируются при первом открытии (один pass по файлу, строится offset map)
- `LazyVStack` рендерит только видимые секции, подгружает содержимое по offset
- Добавлена панель чанков в `DetailView` для больших документов: список чанков со статусами, превью содержимого, возможность перезапуска
- Пагинация: навигация по страницам (N секций на страницу) для очень больших документов

## Capabilities

### New Capabilities
- `mmap-file-loading`: Memory-mapped чтение файлов с ленивой подгрузкой секций
- `section-indexing`: Индексация Markdown секций с offsets для быстрого доступа
- `chunk-list-panel`: Панель чанков со статусами и превью в DetailView
- `pagination`: Постраничная навигация для больших документов

### Modified Capabilities
- `detail-view-content`: Изменение загрузки и отображения контента — с полной загрузки на mmap-based lazy loading

## Impact

- `media2rag-gui/media2rag/Views/DetailView.swift` — полная переработка загрузки и рендера контента
- `media2rag-gui/media2rag/Services/QueueManager.swift` — добавление чанк-информации в `QueueItem`
- `media2rag-gui/media2rag/Models/QueueItem.swift` — новые поля для чанков
- `cli.py` — JSON events для чанк-статусов (chunk_id, chunk_status)
- `processors/chunked_transformer.py` — emit chunk status events
