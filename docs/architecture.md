# Architecture — media2rag v2

## Philosophy

- **Single binary** — everything in one Go binary. No Python runtime, no external deps.
- **Client agnostic** — any client (GUI, web, CLI, bot) communicates via JSON over subprocess or HTTP.
- **Structured events** — every operation emits typed JSON events for progress, streaming, results.
- **No state in client** — GUI is purely presentational. All state, logic, orchestration lives in the binary.
- **Fail fast, fail loud** — errors are values. Malformed input is caught at parse time. No silent failures.

---

## Module Structure

```
media2rag/
├── cmd/                    # CLI entry points (thin wrappers over internal)
│   ├── root.go
│   ├── process.go          # file processing
│   ├── serve.go            # HTTP daemon
│   ├── ask.go              # quick RAG query
│   └── chat.go             # interactive terminal chat
│
├── internal/
│   ├── config/             # configuration loading
│   ├── model/              # domain types (shared across all packages)
│   ├── extract/            # file content extraction
│   ├── pipeline/           # CTG pipeline
│   ├── rag/                # RAG engine
│   ├── chat/               # chat session management
│   ├── coach/              # coaching engine
│   ├── memory/             # persistent memory store
│   ├── llm/                # LLM clients (Ollama, OpenRouter)
│   ├── store/              # data stores (LanceDB, workspace FS)
│   ├── api/                # HTTP server and handlers
│   └── events/             # JSON event emitter
│
├── go.mod
├── go.sum
└── Makefile
```

## Domain Models (`internal/model/`)

These types are shared across all packages. No circular dependencies.

```go
// ExtractedContent — raw output from any extractor
type ExtractedContent struct {
    Title     string
    Author    string
    Source    string            // original file path or URL
    DocType   string            // pdf, epub, video, audio, image, markdown
    Content   string            // plain text
    Language  string
    Sections  []Section
    Images    []ExtractedImage
    Metadata  map[string]string
    WordCount int
    CharCount int
}

type Section struct {
    Heading string
    Content string
    Level   int
}

type ExtractedImage struct {
    Path    string
    Caption string
    Data    []byte // base64 or raw bytes for vision models
}

// RAGDocument — final document with frontmatter
type RAGDocument struct {
    Markdown string
    Metadata DocumentMetadata
}

type DocumentMetadata struct {
    Title         string
    Author        string
    Source        string
    DocType       string
    Language      string
    Domains       []string
    CoreThesis    string
    MentalModels  []string
    Claims        []Claim
    Takeaways     []string
    KeyTerms      []KeyTerm
    Summary       string
    KeyInsights   []string
    WordCount     int
    Topics        []string
}

type Claim struct {
    Text       string
    Type       string // fact, opinion, prediction
    Confidence float64
}

type KeyTerm struct {
    Term       string
    Definition string
}
```

## Key Interfaces

### Extractor

```go
type Extractor interface {
    // Detect returns true if this extractor can handle the given path/URL.
    Detect(path string) bool
    // Extract returns the raw content from a file/URL.
    Extract(ctx context.Context, path string) (*ExtractedContent, error)
}
```

### Pipeline Stage

```go
// Stage is a single step in the CTG pipeline.
type Stage interface {
    Name() string
    Process(ctx context.Context, input string, emitter EventEmitter) (string, error)
}
```

### LLM Client

```go
type LLMClient interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error)
    Embed(ctx context.Context, text string) ([]float32, error)
}

type ChatRequest struct {
    Model     string
    Messages  []Message
    Stream    bool
    Reasoning bool
    Images    []string // base64-encoded
}

type ChatResponse struct {
    Content   string
    Reasoning string
    Model     string
}

type StreamDelta struct {
    Content   string
    Reasoning string
}

type Message struct {
    Role    string // system, user, assistant
    Content string
}
```

### Vector Store — Qdrant

**Решение:** Qdrant (Rust-бинарник, HTTP/gRPC API).

**Почему не LanceDB:**
- LanceDB требует Python runtime (lancedb_server.py)
- Qdrant — Rust-бинарник, запускается без Python
- Go клиент: `github.com/qdrant/go-client`
- Production-ready, используется в production

**Интеграция:**
- Qdrant запускается как отдельный процесс (systemd на VDS, brew на macOS)
- Go общается через gRPC (qdrant/go-client)
- Hybrid search: dense vectors + sparse vectors (BM25 через Qdrant's sparse indexing)

```go
type VectorStore interface {
    UpsertPoints(ctx context.Context, collection string, points []Point) error
    SearchPoints(ctx context.Context, req SearchRequest) ([]SearchResult, error)
    DeletePoints(ctx context.Context, collection string, ids []string) error
    ListCollections(ctx context.Context) ([]CollectionInfo, error)
}
```

### RAG Engine

```go
type RAGEngine struct {
    store VectorStore
    llm   LLMClient
}

type RAGQuery struct {
    Question   string
    TopK       int
    DocumentID string // optional filter
    Rewrite    bool
    Rerank     bool
}

type RAGResponse struct {
    Answer      string
    Sources     []SourceChunk
    Confidence  Confidence
}

type SearchResult struct {
    Chunk       Chunk
    Score       float64
}
```

### Memory Store

```go
type MemoryStore interface {
    Store(ctx context.Context, userID string, entry MemoryEntry) error
    Search(ctx context.Context, userID string, query string, topK int) ([]MemoryEntry, error)
    Delete(ctx context.Context, userID string, entryID string) error
    List(ctx context.Context, userID string) ([]MemoryEntry, error)
}

type MemoryEntry struct {
    ID        string
    UserID    string
    Content   string
    Category  string // fact, preference, goal
    CreatedAt time.Time
}
```

### Event Emitter

```go
type EventEmitter interface {
    Emit(event Event)
    Done()
}

type Event struct {
    Type      string  `json:"type"`
    Data      any     `json:"data,omitempty"`
    Progress  float64 `json:"progress,omitempty"`
    Error     string  `json:"error,omitempty"`
}

// Event types
const (
    EventExtracting        = "extracting"
    EventExtracted         = "extracted"
    EventCleaningPart      = "cleaning_part"
    EventCleaningDone      = "cleaning_done"
    EventMapChunk          = "map_chunk"
    EventMapChunkDone      = "map_chunk_done"
    EventMapDone           = "map_done"
    EventMergeSubsection   = "merge_subsection"
    EventReduceDone        = "reduce_done"
    EventGenerating        = "generating"
    EventGenerated         = "generated"
    EventCompleted         = "completed"
    EventError             = "error"
    EventLLMToken          = "llm_token"
)
```

---

## Communication Models

### Subprocess Mode (File Processing)

```
Client                     media2rag process --json
  │                              │
  │  $ media2rag process file.txt --json
  │─────────────────────────────>│
  │                              │
  │  {"type":"extracting",...}   │
  │<─────────────────────────────│
  │  {"type":"cleaning_part",...}│
  │<─────────────────────────────│
  │  ...                         │
  │  {"type":"llm_token",        │
  │   "data":{"tokens":"..."}}   │
  │<─────────────────────────────│
  │  {"type":"completed",        │
  │   "data":{"output":"..."}}   │
  │<─────────────────────────────│
```

- Each line is a complete JSON object (newline-delimited)
- Client reads stdout line by line
- One process per file

### HTTP Mode (Chat, RAG, Coaching)

```
Client                     media2rag serve
  │                              │
  │  POST /api/query             │
  │  {"question":"..."}          │
  │─────────────────────────────>│
  │                              │
  │  202 Accepted                │
  │  {"session_id":"abc123"}     │
  │<─────────────────────────────│
  │                              │
  │  WS /api/stream/abc123       │
  │─────────────────────────────>│
  │                              │
  │  {"type":"token",            │
  │   "data":"ответ..."}         │
  │<─────────────────────────────│
  │  {"type":"sources", ...}     │
  │<─────────────────────────────│
  │  {"type":"done"}             │
  │<─────────────────────────────│
```

- Long-running server process
- WebSocket for streaming LLM responses
- REST for CRUD operations

---

## Error Handling

```go
var (
    ErrExtractionFailed  = errors.New("extraction failed")
    ErrPipelineFailed    = errors.New("pipeline processing failed")
    ErrLLMUnavailable    = errors.New("LLM service unavailable")
    ErrStoreUnavailable  = errors.New("store unavailable")
    ErrConfigInvalid     = errors.New("invalid configuration")
    ErrNotFound          = errors.New("not found")
    ErrTimeout           = errors.New("operation timed out")
)
```

- All errors are sentinel values (wrap with context)
- HTTP API returns structured JSON errors: `{"error": {"code": "...", "message": "..."}}`
- Subprocess mode: error events before process exits with code 1

---

## Configuration

```go
type Config struct {
    // LLM Backend
    LLM struct {
        DefaultBackend string // "ollama" or "openrouter"
        OllamaURL      string // default: http://localhost:11434
        EmbeddingURL   string // default: http://localhost:11435
        OpenRouterKey  string
        OpenRouterURL  string // default: https://openrouter.ai/api/v1
        DefaultModel   string
    }

    // Vector Store
    LanceDB struct {
        URL      string // default: http://localhost:54321
        Timeout  Duration
    }

    // Processing
    Pipeline struct {
        ChunkSize     int    // default: 8000 chars
        ChunkOverlap  int    // default: 200
        TargetTokens  int    // default: 512
        OverlapTokens int    // default: 64
        ExtractOnly   bool
        QualityCheck  bool
    }

    // Workspace
    Workspace struct {
        Directory string // default: ~/.media2rag/workspace
    }

    // Server
    Server struct {
        Host string // default: localhost
        Port int    // default: 8542
    }
}
```

Loaded from:
1. `.env` in current directory
2. `~/.media2rag/config.yaml`
3. CLI flags (highest priority)

---

### CTG Pipeline

Pipeline спроектирован как цепочка маленьких, фокусных этапов. Каждый этап — отдельная функция с context, error handling, event emitter. Никаких "универсальных" промптов. Подробно: [ctg-pipeline.md](ctg-pipeline.md)

---

## Data Flow: Full File Processing

```
File path
   │
   ▼
Extractor.Detect → match by extension
   │
   ▼
Extractor.Extract → ExtractedContent
   │
   ├── text, sections, metadata
   │
   ▼
Pipeline.Run
   │
   ├── Compressor
   │   └── LLM clean + artifact removal → clean text
   │
   ├── Transformer (Semantic Chunker)
   │   ├── Split into semantic chunks
   │   ├── Map: LLM process each chunk (topics, claims, entities)
   │   └── Reduce: LLM merge + dedup sections
   │
   ├── Generator
   │   └── Assemble RAGDocument (markdown + frontmatter)
   │
   └── (optional) Quality Check
       └── LLM evaluate → pass/fail + revision
   │
   ▼
Save RAGDocument to workspace
   │
   ▼
Index into LanceDB (via store package → lancedb_server)
   │
   ▼
Completed event → client
```

---

## Data Flow: Chat with RAG

```
User message
   │
   ▼
Chat session → load history + context
   │
   ▼
Memory.Search → relevant memories
   │
   ▼
RAGEngine.Query
   │
   ├── Rewrite (format → classify → semantic rewrite)
   ├── Hybrid search (BM25 + semantic via LanceDB)
   ├── LLM rerank (optional)
   ├── Parent lookup
   ├── Dedup
   └── Build context
   │
   ▼
LLM.StreamChat (system + history + context + question)
   │
   ├── token → client (WebSocket)
   ├── reasoning → client (if available)
   └── sources → client (at end)
   │
   ▼
Optional: Memory.Store (if LLM emitted memory block)
```
