# AGENTS.md — media2rag (Go)

## What this is

Go binary that converts any content (URL, Markdown, files) into RAG-ready Markdown with structured metadata. Single binary, zero runtime dependencies.

## Developer commands

```bash
go build ./cmd/media2rag     # build binary
./media2rag process <url>    # process URL via rdrr
./media2rag process ./file.md # process local Markdown
./media2rag serve            # start HTTP daemon
```

## Architecture

**Single binary, all logic inside.** See `docs/` for detailed design:
- `architecture.md` — interfaces, communication, data flow
- `ctg-pipeline.md` — Compress → Split → Process → Assemble
- `extraction.md` — all formats → Markdown via rdrr + Go-native
- `llm-clients.md` — Ollama + OpenAI-compatible providers

## Key directories

| Path | Purpose |
|------|---------|
| `cmd/media2rag/` | CLI entry points |
| `internal/` | All internal packages |
| `docs/` | Design documents |

## Conventions

- Pure Go, no CGo unless required
- Config: `~/.media2rag/config.yaml` + CLI flags + env vars
- Vector store: Qdrant (gRPC/HTTP)
- LLM: Ollama (local) or OpenAI-compatible (OpenRouter, OpenAI, Groq...)
