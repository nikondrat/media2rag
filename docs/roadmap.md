# Roadmap: media2rag v2 (Go)

## Видение

Единый Go-бинарник `media2rag` — ядро системы. Вся логика обработки, LLM-взаимодействия, RAG-поиска, чата и коучинга живёт в нём. Любой клиент (GUI, web, CLI, Telegram-бот) — тонкая обёртка, которая вызывает бинарник и рендерит результат.

**Мотивация:**
- Python-версия — прототип: дублирование логики в CLI и GUI, хрупкий CTG пайплайн, проблемы с зависимостями и деплоем
- Go даёт единый бинарник, статическую типизацию, goroutines для стриминга, кросс-компиляцию, никаких runtime-зависимостей

**Принципы:**
- Zero-инфраструктура для деплоя: скопировал бинарник на VDS → работает
- Клиент ничего не знает о логике — только вызывает и рендерит
- Все операции стримят структурированные JSON-события
- Graceful error handling — никаких `NoneType has no attribute` в рантайме

---

## Архитектура

```
┌──────────────────────────────────────────────────────────┐
│  Клиенты (тонкие)                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐  │
│  │ macOS GUI│  │   Web    │  │   CLI    │  │TG Bot   │  │
│  │ (Swift)  │  │ (React)  │  │ (iterm2) │  │ (Go)    │  │
│  └─────┬────┘  └────┬─────┘  └────┬─────┘  └────┬────┘  │
│        │            │              │              │       │
│        ├── subprocess (--json) ────┘              │       │
│        └── HTTP REST + WebSocket ─────────────────┘       │
└──────────────────────────┬───────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────┐
│  media2rag (Go) — единый бинарник                        │
│                                                          │
│  ┌────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  CLI Mode  │  │  Serve Mode  │  │  Shared Core     │  │
│  │            │  │              │  │                  │  │
│  │• process   │  │• POST /proc  │  │• extraction      │  │
│  │• ask       │  │• GET /chat   │  │• pipeline (CTG)  │  │
│  │• chat      │  │• POST /query │  │• RAG engine      │  │
│  │• extract   │  │• WS /stream  │  │• LLM clients     │  │
│  │            │  │• POST /coach │  │• memory          │  │
│  └────────────┘  └──────────────┘  └──────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  External Services                                 │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │  │
│  │  │  Ollama  │  │OpenRouter│  │  Qdrant           │  │  │
│  │  │ localhost│  │  API     │  │  localhost:6334   │  │  │
│  │  │ :11434   │  │  HTTP    │  │  (Rust бинарник)  │  │  │
│  │  └──────────┘  └──────────┘  └──────────────────┘  │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```
┌──────────────────────────────────────────────────────────┐
│  Клиенты (тонкие)                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐  │
│  │ macOS GUI│  │   Web    │  │   CLI    │  │TG Bot   │  │
│  │ (Swift)  │  │ (React)  │  │ (iterm2) │  │ (Go)    │  │
│  └─────┬────┘  └────┬─────┘  └────┬─────┘  └────┬────┘  │
│        │            │              │              │       │
│        ├── subprocess (--json) ────┘              │       │
│        └── HTTP REST + WebSocket ─────────────────┘       │
└──────────────────────────┬───────────────────────────────┘
                           │
┌──────────────────────────▼───────────────────────────────┐
│  media2rag (Go) — единый бинарник                        │
│                                                          │
│  ┌────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  CLI Mode  │  │  Serve Mode  │  │  Shared Core     │  │
│  │            │  │              │  │                  │  │
│  │• process   │  │• POST /proc  │  │• extraction      │  │
│  │• ask       │  │• GET /chat   │  │• pipeline (CTG)  │  │
│  │• chat      │  │• POST /query │  │• RAG engine      │  │
│  │• extract   │  │• WS /stream  │  │• LLM clients     │  │
│  │            │  │• POST /coach │  │• memory          │  │
│  └────────────┘  └──────────────┘  └──────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  External Services                                 │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │  │
│  │  │  Ollama  │  │OpenRouter│  │  LanceDB Server   │  │  │
│  │  │ localhost│  │  API     │  │  localhost:54321  │  │  │
│  │  │ :11434   │  │  HTTP    │  │  (Python процесс) │  │  │
│  │  └──────────┘  └──────────┘  └──────────────────┘  │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### Режимы работы

| Режим | Команда | Для чего |
|-------|---------|----------|
| **Subprocess** | `media2rag process file.txt --json` | Обработка файлов — однократный запуск, JSON события в stdout |
| **Subprocess** | `media2rag ask "вопрос" --json` | Быстрый RAG-запрос |
| **Serve** | `media2rag serve` | Долгоживущий HTTP/WS сервер для чата, RAG, коучинга |
| **CLI Interactive** | `media2rag chat` | Интерактивный чат в терминале |

### Коммуникация

**Subprocess mode (file processing, одноразовые запросы):**
- Go пишет JSON-строки в stdout, каждая на новой строке
- Клиент читает stdout, парсит JSON, обновляет UI
- Типы событий: `progress`, `cleaning_part`, `map_chunk`, `llm_token`, `completed`, `error`

**Serve mode (чат, стриминг):**
- HTTP REST запросы
- WebSocket для стриминга LLM-ответов
- Пул LLM-соединений, keep-alive

---

## Компоненты системы

### 1. Extraction — извлечение контента

Выбор стратегии — **Python subprocess на старте, миграция в Go native по мере необходимости**.

| Тип | Текущее решение (Python) | Go v1 | Go v2 |
|-----|--------------------------|-------|-------|
| PDF | PyMuPDF | `python -m extractors.pdf` | `pdfcpu` / `unipdf` |
| EPUB | ebooklib | `python -m extractors.epub` | `go-epub` native |
| Видео | yt-dlp + whisper | `yt-dlp` subprocess + `whisper` subprocess | `whisper.cpp` CGo |
| Аудио | whisper | `whisper` subprocess | `whisper.cpp` CGo |
| Изображение | Ollama vision | Ollama vision HTTP | Ollama vision HTTP |
| URL/Telegram | yt-dlp / telethon | yt-dlp subprocess | `gotd` native |

На выходе — единая структура `ExtractedContent`:
```go
type ExtractedContent struct {
    Title       string
    Author      string
    Source      string
    Content     string   // чистый текст
    Language    string
    Sections    []Section
    Images      []Image
    Metadata    map[string]string
}
```

### 2. CTG Pipeline — Compress → Transform → Generate

Чистый pipeline из Go-функций:

```go
type Pipeline struct {
    stages []Stage
}

type Stage func(ctx context.Context, input string, emitter EventEmitter) (string, error)
```

**Compressor:**
- LLM-чистка текста (удаление таймстемпов, мусора, рекламы, промо)
- Контекстное окно: если текст > лимита, разбивается на части
- Эмитит: `cleaning_part`, `cleaning_part_done`, `compression_done`

**Transformer (Semantic Chunker + Map-Reduce):**
- Разбивка на семантические чанки
- LLM-обработка каждого чанка (извлечение тем, сущностей, связей)
- Слияние и дедупликация
- Эмитит: `map_chunk`, `map_chunk_done`, `merge_subsection`, `reduce_done`

**Generator:**
- Сборка финального Markdown с YAML frontmatter
- Quality check (опционально)
- Эмитит: `generation_start`, `generation_done`

### 3. RAG Engine — поиск и синтез

```go
type RAGEngine struct {
    vectorStore VectorStore    // Qdrant client
    llm         LLMClient
    memory      MemoryStore
}
```

- **Query Rewrite**: format → complexity → semantic rewrite + multi-query + HyDE
- **Hybrid Search**: BM25 + semantic (RRF merge)
- **LLM Rerank**: опциональная переранжировка LLM
- **Parent Lookup**: замена child-чанков на parent для контекста
- **Dedup**: SHA-256 дедупликация
- **Memory**: добавление релевантных воспоминаний в контекст

### 4. LLM Clients

```go
type LLMClient interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamDelta, error)
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

- **OllamaClient**: HTTP к localhost:11434
- **OpenRouterClient**: HTTP к API с ключом
- Фоллбэк: Ollama → OpenRouter если нет ответа

### 5. Chat + Coaching

**Chat:**
- Управление сессиями (создание, история, контекст)
- RAG-обогащение контекста
- Memory store (личные факты о пользователе)
- Стриминг LLM-ответов

**Coaching:**
- Фазы: discovery → clarity → planning → active → completed
- Action plan management
- Check-in reminders
- Insight generation

### 6. Memory Store

Личная память пользователя (факты, предпочтения, цели):
- SQLite-бэкенд
- CRUD: store, recall, delete, search
- Интеграция с RAG-контекстом

---

## План реализации (фазы)

### Фаза 1: Core Engine (Ядро)
- [ ] Go-модуль + структура проекта
- [ ] Система конфигурации (env + yaml)
- [ ] Event emitter (JSON логгер)
- [ ] LLM clients (Ollama + OpenRouter)
- [ ] SQLite store (сессии, память, настройки) + схема БД
- [ ] Qdrant client + инициализация коллекций
- [ ] `process` команда
- [ ] Экстракторы (rdrr для URL, local Markdown)
- [ ] CTG pipeline (Compress + Transform + Generate)
- [ ] JSON event streaming
- [ ] Progress reporting
- [ ] Работа с workspace
- [ ] RAG engine (hybrid search, rerank, parent lookup)
- [ ] Chat API + session management
- [ ] Auth middleware (API key)
- [ ] Observability (structured logging, tracing, metrics)
- [ ] Test strategy (mock LLM, golden files)

### Фаза 2: Skill System
- [ ] Skill loader (YAML configs)
- [ ] Domain-specific prompts per skill
- [ ] Pipeline config per skill
- [ ] Memory schema per skill
- [ ] Built-in skills: sales analysis, business coach, legal review
- [ ] Skill CLI: `media2rag skills list/install/enable`

### Фаза 3: Coaching Engine
- [ ] `docs/coaching.md` — полный дизайн
- [ ] Phase management (discovery → clarity → planning → active → completed)
- [ ] Action plan data model
- [ ] Check-in reminders
- [ ] Insight generation
- [ ] Coaching API endpoints

### Фаза 4: Marketplace MVP
- [ ] Asset packaging format
- [ ] Upload/download skills and knowledge bases
- [ ] Licensing model
- [ ] Marketplace UI (web)
- [ ] Creator documentation

### Фаза 5: Go-native Extraction (Optional)
- [ ] PDF: pdfcpu / unipdf
- [ ] EPUB: go-epub
- [ ] Audio/Video: whisper.cpp CGo
- [ ] Telegram: gotd native

---

## Структура Go-проекта

```
media2rag/
├── cmd/
│   ├── root.go
│   ├── process.go
│   ├── serve.go
│   ├── ask.go
│   └── chat.go
├── internal/
│   ├── extract/
│   │   ├── extractor.go        // Extractor interface
│   │   ├── url.go              // rdrr-based URL extraction
│   │   ├── local.go            // local file extraction
│   │   └── types.go            // ExtractedContent
│   ├── pipeline/
│   │   ├── pipeline.go         // Pipeline orchestrator
│   │   ├── compressor.go       // LLM clean + dedup
│   │   ├── transformer.go      // Map-reduce chunks
│   │   └── generator.go        // RAGDocument assembly
│   ├── rag/
│   │   ├── engine.go           // RAGEngine
│   │   ├── rewrite.go          // Query rewriting
│   │   ├── search.go           // Hybrid search
│   │   └── rerank.go           // LLM reranking
│   ├── chat/
│   │   ├── session.go          // Chat session
│   │   └── handler.go          // Message handling
│   ├── coach/
│   │   ├── session.go          // Coaching session
│   │   ├── phases.go           // Phase management
│   │   └── prompts.go          // Prompt templates
│   ├── skills/
│   │   ├── loader.go           // Skill YAML loader
│   │   ├── registry.go         // Skill registry
│   │   └── types.go            // Skill data model
│   ├── memory/
│   │   └── store.go            // SQLite-backed memory
│   ├── llm/
│   │   ├── client.go           // LLMClient interface
│   │   ├── ollama.go           // Ollama implementation
│   │   └── openrouter.go       // OpenRouter implementation
│   ├── store/
│   │   └── qdrant.go           // Qdrant client
│   ├── api/
│   │   ├── router.go           // HTTP routes
│   │   ├── process.go          // /api/process
│   │   ├── chat.go             // /api/chat
│   │   ├── query.go            // /api/query
│   │   └── coach.go            // /api/coach
│   ├── events/
│   │   └── emitter.go          // JSON event emitter
│   ├── config/
│   │   └── config.go           // Configuration
│   └── observe/
│       ├── log.go              // Structured logging
│       ├── trace.go            // Request tracing
│       └── metrics.go          // Metrics collection
├── skills/                     // Built-in skills
│   ├── sales-analysis/
│   ├── business-coach/
│   └── legal-review/
├── go.mod
├── go.sum
└── Makefile
```

---

## Референсы

- Текущая Python codebase: `~/dev/tools/transcripts`
- Текущее GUI: `~/dev/tools/media2rag-gui`
- Qdrant: localhost:6334 (gRPC), localhost:6333 (HTTP)
- Ollama: localhost:11434 (LLM + embeddings)
- rdrr: `npx rdrr` (URL extraction)
