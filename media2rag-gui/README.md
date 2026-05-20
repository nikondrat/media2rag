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

This project uses [Tuist](https://tuist.dev) for project generation.

### Setup

1. Install Tuist: `brew install tuist`
2. Generate the Xcode workspace: `tuist generate`
3. Open `media2rag.xcworkspace` in Xcode
4. Build and run

### For new developers

```bash
# Clone the repository
git clone <repo-url>
cd media2rag-gui

# Install Tuist
brew install tuist

# Generate workspace
tuist generate

# Open in Xcode and build
open media2rag.xcworkspace
```

> **Note:** Do not commit `media2rag.xcworkspace`, `media2rag.xcodeproj`, or `Derived/` — they are generated locally.
> Only `Project.swift`, `Tuist.swift`, and `Tuist/Package.swift` are source-controlled.

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
