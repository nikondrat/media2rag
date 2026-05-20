## 1. Model updates

- [x] 1.1 Add `ChunkInfo` struct with id, status, wordCount, errorMessage fields
- [x] 1.2 Add `ChunkStatus` enum (queued, processing, done, error, skipped)
- [x] 1.3 Add `chunks: [ChunkInfo]` field to `QueueItem`
- [x] 1.4 Add `SectionIndex` struct with title, offset, length, level fields

## 2. Memory-mapped file reader

- [x] 2.1 Create `MmapFileReader` class wrapping `FileHandle.map(offset:length:)`
- [x] 2.2 Implement `scanSections()` method to build `SectionIndex[]` from mapped data
- [x] 2.3 Implement `getSectionContent(index:)` method using offset ranges
- [x] 2.4 Add section content cache (`[Int: String]`)
- [x] 2.5 Implement proper cleanup (close FileHandle on deinit)
- [x] 2.6 Add fallback to `read(upToCount:)` when mmap fails

## 3. DetailView lazy rendering

- [x] 3.1 Replace `String(contentsOf:)` with `MmapFileReader` in DetailView
- [x] 3.2 Replace `ScrollView + ForEach` with `LazyVStack` for formatted preview
- [x] 3.3 Update `parseMarkdownSections` to work with offset-based section retrieval
- [x] 3.4 Add section caching to avoid re-parsing on scroll
- [x] 3.5 Keep raw/intermediate modes using full file load (no change needed)

## 4. Pagination

- [ ] 4.1 Add `sectionsPerPage = 50` constant and page calculation logic
- [ ] 4.2 Add pagination state (`currentPage`, `totalPages`) to DetailView
- [ ] 4.3 Create `PageNavigation` view with prev/next buttons and page numbers
- [ ] 4.4 Show pagination controls only when sections > threshold (200)
- [ ] 4.5 Reset scroll position on page change

## 5. Chunk list panel

- [ ] 5.1 Create `ChunkListView` showing list of chunks with status icons
- [ ] 5.2 Create `ChunkRow` view with status, word count, error message
- [ ] 5.3 Add chunk content preview panel (click to view chunk markdown)
- [ ] 5.4 Add retry button for failed chunks
- [ ] 5.5 Show chunk list tab/section in DetailView for map-reduce documents

## 6. CLI chunk events

- [x] 6.1 Update `QueueManager.processItem` to parse chunk events and populate `QueueItem.chunks`
- [x] 6.2 Add `chunk_id` field to CLI JSON events (`map_chunk`, `map_chunk_done`, `map_chunk_error`)
- [x] 6.3 Update `ChunkedTransformer` to emit chunk_id in events
