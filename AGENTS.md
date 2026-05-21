# AGENTS.md — media2rag

## What this is

Python tool that converts PDF, EPUB, video, audio, and images into RAG-ready Markdown with structured metadata (frontmatter, topics, key insights).

## Developer commands

```bash
uv sync                     # install deps (Python >=3.11)
uv run cli.py <file>        # process single file
uv run cli.py <dir> --batch # batch process directory
uv run cli.py <file> --extract-only  # skip LLM, raw extraction only
uv run cli.py <file> --backend ollama --model <name>
uv run cli.py <file> -o ./output-dir
uv run bookfinder.py <isbn|url|name>  # find/download books via LibGen
uv run main.py [channel_url]          # legacy: download YouTube transcripts via npx rdrr
```

## Prerequisites

- `brew install ffmpeg` — required for audio/video processing
- `brew install tesseract` — required for PDF OCR fallback
- `ollama pull qwen3.5:27b` — default local model (or set `OLLAMA_CTG_MODEL`)
- Ollama server must be running at `http://localhost:11434` for local backend
- `OPENROUTER_API_KEY` env var for cloud backend (also accepts `OPENROUTER_API`)

## Architecture

**Entry points:**
- `cli.py` — main CLI (`media2rag` entrypoint). Parses args, routes to extractors.
- `main.py` — legacy YouTube transcript downloader (uses `npx rdrr`). Not part of cli.py flow.
- `bookfinder.py` — standalone book finder (LibGen/Anna's Archive). Separate tool.

**Processing flow (cli.py):**
1. **Extractor** selects by file extension → produces `ExtractedContent`
   - `extractors/pdf_epub_extractor.py` — PyMuPDF for PDF (with OCR fallback), ebooklib for EPUB (with image extraction)
   - `extractors/video_extractor.py` — yt-dlp + whisper
   - `extractors/audio_extractor.py` — whisper
   - `extractors/image_extractor.py` — Ollama vision (only with Ollama backend)
   - `extractors/markdown_extractor.py` — passthrough
2. **CTG Pipeline** (`processors/ctg_pipeline.py`) — Compressor → Transformer → Generator
3. **Output** — `RAGDocument` saved as Markdown with YAML frontmatter to `output/`

**LLM backends:**
- Default: Ollama (local). Falls back from OpenRouter if API key missing/unavailable.
- `config.py` loads from `.env` via `python-dotenv`. `AppConfig.from_env()` is the single config source.

## Key directories

| Path | Purpose |
|------|---------|
| `extractors/` | File-type-specific content extractors |
| `processors/` | CTG pipeline (compress, transform, generate) |
| `clients/` | LLM clients (Ollama, OpenRouter) |
| `domain/` | Domain models (`RAGDocument`, `ExtractedContent`) |
| `output/` | Default output directory (gitignored) |
| `transcripts/` | Legacy output from `main.py` (gitignored) |
| `_done.json` | Tracks processed YouTube video IDs for `main.py` |

## Conventions

- No tests exist. No CI/CD. No linter/formatter configured.
- Config uses `dataclass` + `os.getenv` (no pydantic validation).
- `.env` is gitignored but committed in this repo — contains real API key. Do not expose.
- Image extraction only works with Ollama backend (requires vision model).
- Whisper defaults to `cpu` — set `WHISPER_DEVICE` for cuda/mps.

## Maintaining Project Context

**`PROJECT_CONTEXT.md`** — единый источник контекста для AI-ассистентов. Обновляй после:
- Добавления новых экстракторов/процессоров
- Изменения архитектуры (новые модули, удаление старых)
- Смены принципов/конвенций
- Значимых изменений в потоке данных

**Правило:** Изменил код → проверь, устарел ли `PROJECT_CONTEXT.md` → обнови секции при необходимости.
