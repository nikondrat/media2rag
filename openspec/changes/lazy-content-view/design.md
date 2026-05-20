## Context

`DetailView` загружает весь Markdown файл через `String(contentsOf: url)` при открытии. Для файлов 10MB+ (50K+ слов) это:
- Блокирует main thread при чтении
- Занимает 10MB+ RAM на документ
- `parseMarkdownSections()` парсит весь файл перед рендером
- Пользователь не видит прогресс чанков при map-reduce обработке

## Goals / Non-Goals

**Goals:**
- Memory-mapped чтение — в RAM только видимые секции
- Быстрая индексация секций (offset map) при первом открытии
- LazyVStack рендерит только видимые секции
- Панель чанков со статусами, превью, retry
- Пагинация для очень больших документов

**Non-Goals:**
- Изменение CLI pipeline логики
- Изменение формата чанков
- Редактирование контента из GUI (read-only просмотр)

## Decisions

### 1. FileHandle.map() for memory-mapped I/O

Swift `FileHandle.map(offset:length:)` возвращает `Data` без загрузки файла в RAM. OS подгружает страницы по demand.

```swift
let fh = FileHandle(forReadingFrom: url)
let fileData = fh.map(offset: 0, length: fileSize)
// fileData — виртуальный доступ, RAM = 0 пока не читаешь
let sectionData = fileData.subdata(in: offset..<offset+length)
```

**Alternatives considered:**
- `String(contentsOf:)` — текущий подход, жрёт память
- `FileHandle.read(upToCount:)` — ручной seek, сложнее
- `mmap()` через C interop — lower-level, но FileHandle.map() достаточно

### 2. Section index structure

Один pass по `fileData` для построения индекса:
```swift
struct SectionIndex {
    let title: String
    let offset: Int
    let length: Int
    let level: Int  // heading level
}
```

Индекс строится быстро (scan по `#` символам), занимает ~KB даже для больших файлов.

### 3. LazyVStack + pagination

`LazyVStack` рендерит только видимые элементы. Для очень больших документов (>200 секций) — разбивка на страницы по 50 секций:
- `Page 1: sections 0-49`
- `Page 2: sections 50-99`
- Navigation: `← 1 2 3 ... 12 →`

### 4. Chunk list panel

Отдельная tab/section в `DetailView`:
```
┌─ Chunks (12) ──────────────────────────────┐
│ ✓ chunk_001  2.3K words  Done              │
│ ✓ chunk_002  1.8K words  Done              │
│ ⟳ chunk_003  3.1K words  Processing...     │
│ ○ chunk_004  2.7K words  Queued            │
│ ✗ chunk_005  1.9K words  Error: timeout    │
│                                            │
│ [View Content] [Retry Failed] [Clear All]  │
└────────────────────────────────────────────┘
```

Данные чанков приходят из CLI JSON events (`map_chunk`, `map_chunk_done`, `map_chunk_error`). `QueueItem` хранит `chunks: [ChunkInfo]`.

### 5. ChunkInfo model

```swift
struct ChunkInfo: Identifiable {
    let id: Int           // chunk index
    var status: ChunkStatus
    var wordCount: Int?
    var errorMessage: String?
    var contentOffset: Int?  // offset in chunk file for mmap
    var contentLength: Int?
}
enum ChunkStatus { case queued, processing, done, error, skipped }
```

### 6. Content loading strategy

```
onAppear:
  1. FileHandle.map() → Data (RAM = 0)
  2. Scan for headings → SectionIndex[] (fast, ~50ms for 10MB)
  3. Store index, release full mmap if needed
  
onScroll (LazyVStack):
  4. For each visible section:
     mmapData.subdata(in: offset..<offset+length) → String
     Render as formatted view
```

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| `FileHandle.map()` может fail на файловых системах без mmap | Fallback на `read(upToCount:)` |
| Индексация неточная для сложного Markdown | Парсер уже есть (`parseMarkdownSections`), переиспользовать |
| Chunk retry требует CLI rerun | Retry запускает CLI только для конкретного чанка |
| LazyVStack может мерцать при быстром скролле | `.id()` на секциях, кэш загруженных секций |
