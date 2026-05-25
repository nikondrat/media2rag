# media2rag

Single Go binary that converts any content into RAG-ready Markdown.

## Quick start

```bash
# Build
go build -o media2rag ./cmd/media2rag

# Process a URL (web page, YouTube, Telegram, GitHub — anything)
./media2rag process https://example.com/article

# Process a local Markdown file
./media2rag process ./notes.md

# Start HTTP daemon for chat/RAG/coaching
./media2rag serve
```

## Requirements

- Go 1.22+
- `npx rdrr` for URL processing (`npm i -g rdrr`)
- Ollama or OpenAI-compatible API key
- Qdrant for vector storage

## Docs

See `docs/` for architecture and design.
