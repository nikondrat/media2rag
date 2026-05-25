## 1. Project Setup

- [x] 1.1 Initialize Go module: `go mod init media2rag`
- [x] 1.2 Create directory structure: `cmd/media2rag/`, `internal/config/`, `internal/model/`, `internal/events/`, `internal/llm/`
- [x] 1.3 Add dependencies: `cobra`, `viper`, `godotenv`
- [x] 1.4 Create `Makefile` with `build`, `run`, `test` targets

## 2. Domain Models (`internal/model/`)

- [x] 2.1 Create `types.go` with `ExtractedContent`, `Section`, `ExtractedImage`
- [x] 2.2 Add `RAGDocument`, `DocumentMetadata`, `Claim`, `KeyTerm`
- [x] 2.3 Add LLM models: `ChatRequest`, `ChatResponse`, `StreamDelta`, `Message`
- [x] 2.4 Add `Event` struct with JSON tags
- [x] 2.5 Add `MemoryEntry` struct
- [x] 2.6 Add error sentinels: `ErrExtractionFailed`, `ErrLLMUnavailable`, `ErrConfigInvalid`, etc.

## 3. Configuration (`internal/config/`)

- [x] 3.1 Define `Config` struct with LLM, Pipeline, Workspace, Server sections
- [x] 3.2 Implement YAML loading from `~/.media2rag/config.yaml`
- [x] 3.3 Implement `.env` loading with `godotenv`
- [x] 3.4 Implement CLI flag parsing and merge with priority: flags > env > yaml
- [x] 3.5 Add validation: `DefaultBackend` must be "ollama" or "openrouter"
- [x] 3.6 Set defaults: OllamaURL=`http://localhost:11434`, Server.Port=8542

## 4. Event Emitter (`internal/events/`)

- [x] 4.1 Define `EventEmitter` interface: `Emit(Event)`, `Done()`
- [x] 4.2 Implement `StdoutEmitter` — writes JSON-lines to stdout
- [x] 4.3 Add `NoopEmitter` for non-subprocess mode
- [x] 4.4 Test: emit 3 events, verify 3 JSON lines on stdout

## 5. LLM Clients (`internal/llm/`)

- [x] 5.1 Define `LLMClient` interface: `Chat`, `StreamChat`, `Embed`
- [x] 5.2 Implement `OllamaClient` — HTTP to localhost:11434
- [x] 5.3 Implement `OllamaClient.Chat` — POST `/api/chat`
- [x] 5.4 Implement `OllamaClient.Embed` — POST `/api/embed`
- [x] 5.5 Implement `OllamaClient.StreamChat` — SSE parsing
- [x] 5.6 Implement `OpenRouterClient` — OpenAI-compatible API with Bearer auth
- [x] 5.7 Implement fallback: Ollama → OpenRouter on error
- [x] 5.8 Test: `OllamaClient.Chat` with running Ollama returns response

## 6. CLI Skeleton (`cmd/media2rag/`)

- [x] 6.1 Create `main.go` with root cobra command
- [x] 6.2 Add `process.go` — subcommand with file/URL arg + `--json` flag
- [x] 6.3 Add `serve.go` — subcommand with `--host`, `--port` flags
- [x] 6.4 Add `ask.go` — subcommand with question arg + `--json` flag
- [x] 6.5 Add `chat.go` — subcommand for interactive mode
- [x] 6.6 Add global flags: `--config`, `--verbose`
- [x] 6.7 Wire config loading into root command's `PersistentPreRun`
- [x] 6.8 Wire LLM client initialization (Ollama by default)

## 7. Integration

- [x] 7.1 `process` command: load config, create emitter, emit "starting" event, exit
- [x] 7.2 `ask` command: load config, create LLM client, call Chat, print response
- [x] 7.3 `go build ./cmd/media2rag` succeeds
- [x] 7.4 `./media2rag --help` shows all subcommands
- [x] 7.5 `./media2rag ask "hello"` returns LLM response (with Ollama running)
