# media2rag GUI

macOS native application for converting media to RAG-ready Markdown.

## Features

- **Drag & drop** — files or paste URLs
- **Queue management** — process multiple files with progress tracking
- **Real-time preview** — see results as they're generated
- **Concurrent processing** — multiple files in parallel
- **Menu bar app** — run in background
- **Finder integration** — open output files directly

## Supported formats

| Type | Extensions | URLs |
|------|------------|------|
| Documents | PDF, EPUB, MD | |
| Video | MP4, MKV, AVI, MOV, WebM | YouTube, Vimeo |
| Audio | MP3, WAV, M4A, FLAC, OGG, AAC | |
| Images | PNG, JPG, WebP, BMP, TIFF | |
| Web | | Articles, Telegram channels |

## Requirements

- macOS 14+ (Sonoma)
- Python 3.11+ with `media2rag` CLI installed
- Ollama or OpenRouter API key

## Build

1. Open `media2rag.xcodeproj` in Xcode
2. Set `CLI_PATH` build setting to your `cli.py` path
3. Build and run

## Architecture

```
┌─────────────────┐
│   SwiftUI UI    │  ← Native macOS interface
├─────────────────┤
│  QueueManager   │  ← Manages processing queue
├─────────────────┤
│   CLIRunner     │  ← Runs cli.py --json as subprocess
├─────────────────┤
│  media2rag CLI  │  ← Python backend (existing)
└─────────────────┘
```

## JSON Protocol

CLI emits JSON events to stdout:

```json
{"status": "extracting", "file": "video.mp4", "type": "VideoExtractor"}
{"status": "extracted", "file": "video.mp4", "type": "video", "words": 5432}
{"status": "compression_start", "chars": 45000}
{"status": "compressing_chunk", "current": 1, "total": 3}
{"status": "compressed_chunk", "current": 1, "total": 3}
{"status": "compression_done", "chars": 32000}
{"status": "transformation_start"}
{"status": "transformation_done", "topics": ["topic1", "topic2"]}
{"status": "generation_start"}
{"status": "generation_done"}
{"status": "completed", "file": "video.mp4", "output": "/path/output.md"}
{"status": "error", "file": "video.mp4", "message": "..."}
```
