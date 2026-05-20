## 1. Domain model updates

- [x] 1.1 Add `image_paths: list[Path]` field to `ExtractedContent` in `domain/document.py`
- [x] 1.2 Add `workspace_dir` parameter to `RAGDocument.save()` method
- [x] 1.3 Update `RAGDocument.save()` to write to workspace structure (`output/final.md`, `intermediate/raw.md`)

## 2. CLI workspace path resolution

- [x] 2.1 Add `--workspace` argument to `cli.py` argparser
- [x] 2.2 Add `WORKSPACE` env var support in `config.py`
- [x] 2.3 Implement workspace resolution priority: `--workspace` > `WORKSPACE` > `-o` > default
- [x] 2.4 Keep `-o` / `--output` as alias for backward compatibility

## 3. ChunkedTransformer workspace integration

- [x] 3.1 Update `ChunkedTransformer.__init__` to accept `workspace_dir` parameter
- [x] 3.2 Update `_chunk_dir()` to use `workspace_dir / "chunks"` instead of `.chunks/`
- [x] 3.3 Update `_reduce()` to save merged sections to `workspace_dir / "sections"` instead of chunk_dir
- [x] 3.4 Add JSON event `sections_saved` with list of section names

## 4. CTGPipeline workspace integration

- [x] 4.1 Pass `workspace_dir` to `ChunkedTransformer` in `CTGPipeline.process()`
- [x] 4.2 Pass `workspace_dir` to `Compressor` and `Transformer` if needed
- [x] 4.3 Update `emit_json` calls to include `work_dir` field

## 5. Extractor image extraction

- [x] 5.1 Update `PdfEpubExtractor.extract()` to save images to `workspace_dir / "images"`
- [x] 5.2 Populate `ExtractedContent.image_paths` with saved image paths
- [x] 5.3 Add `images` field to JSON completion event

## 6. CLI process_single integration

- [x] 6.1 Create workspace directory structure in `process_single()` (all subdirs)
- [x] 6.2 Pass `workspace_dir` to extractor and pipeline
- [x] 6.3 Update intermediate file path to `intermediate/raw.md`
- [x] 6.4 Update output file path to `output/final.md`
- [x] 6.5 Update JSON events to include `work_dir`, `sections`, `images`

## 7. GUI updates

- [x] 7.1 Update `SettingsManager` to use `workspaceDirectory` instead of `outputDirectory`
- [x] 7.2 Update `QueueManager` to pass `--workspace` instead of `-o` to CLI
- [x] 7.3 Update `QueueItem` to store `workspaceURL` instead of separate `outputURL`/`intermediateURL`
- [x] 7.4 Update `DetailView` to load from new workspace paths
