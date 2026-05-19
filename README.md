# media2rag

Convert PDF, EPUB, video, audio, and images into RAG-ready Markdown with structured metadata.

## Features

- **PDF/EPUB** → Marker OCR + layout parsing
- **Video** → yt-dlp + Whisper transcription
- **Audio** → Whisper transcription
- **Images** → Ollama vision description
- **CTG Pipeline** → Compression → Transformation → Generation
- **RAG-ready output** → Frontmatter, topics, key insights, structured sections

## Installation

```bash
uv sync
brew install ffmpeg  # for audio/video processing
ollama pull gemma4:26b
```

## Usage

```bash
# Single file with OpenRouter (fast, default if API key set)
uv run cli.py path/to/document.pdf

# Specify model
uv run cli.py document.pdf --model tencent/hy3-preview

# Use local Ollama instead
uv run cli.py document.pdf --backend ollama --model gemma4:26b

# Batch process directory
uv run cli.py path/to/folder --batch

# Extract only (no LLM processing)
uv run cli.py podcast.mp3 --extract-only

# Custom output directory
uv run cli.py video.mp4 -o ./rag-docs
```

## Output Format

```markdown
---
title: "How to Scale Your Business"
source: "path/to/video.mp4"
type: "video"
author: "Mikhail Grebenyuk"
topics: ["масштабирование", "бизнес-модель", "продажи"]
summary: "5 actionable steps to scale business revenue..."
key_insights:
  - "Focus on current client base before acquisition"
  - "Implement tiered motivation for sales team"
---

# How to Scale Your Business

## Ключевые принципы
...

## Практические шаги
...
```

## Environment Variables

```bash
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_CTG_MODEL=gemma4:26b
OLLAMA_VISION_MODEL=gemma4:latest
OPENROUTER_API_KEY=sk-or-...
LLM_BACKEND=ollama
OUTPUT_DIR=output
```
