# HTTP API (Serve Mode) — v1 Design

## Запуск

```bash
media2rag serve
# или
media2rag serve --port 8542 --host 0.0.0.0
```

Сервер работает постоянно. Все эндпоинты — REST + SSE для стриминга.

## Эндпоинты

### Process — обработка файла/URL

```
POST /api/process
Content-Type: application/json

{
  "source": "https://youtube.com/watch?v=...",
  "backend": "ollama",
  "model": "qwen3.5:27b",
  "extract_only": false,
  "quality_check": false
}

→ 202 Accepted
{
  "task_id": "abc123",
  "status": "queued"
}
```

**Стриминг прогресса (SSE):**
```
GET /api/stream/abc123

data: {"type":"extracting","progress":0.05}
data: {"type":"cleaning_part","current":1,"total":3,"progress":0.3}
data: {"type":"llm_token","data":{"token":"..."}}
data: {"type":"completed","data":{"output":"./workspace/abc123/output/final.md"}}
```

### Query — быстрый RAG-запрос (без сессии)

```
POST /api/query
Content-Type: application/json

{
  "question": "расскажи про HNSW",
  "top_k": 5,
  "rewrite": true,
  "rerank": true
}

→ 200 OK
{
  "answer": "HNSW это графовый индекс...",
  "sources": [
    {"ref": 1, "title": "...", "source": "..."}
  ]
}
```

**Стриминг ответа (SSE):**
```
GET /api/stream/query/{query_id}

data: {"type":"token","data":"HNSW"}
data: {"type":"token","data":" это"}
data: {"type":"sources","data":[{"ref":1,"title":"..."}]}
event: done
data: {}
```

### Chat — сессии

```
POST /api/sessions
Content-Type: application/json

{
  "title": "HNSW discussion"  // опционально, авто-генерация
}

→ 201 Created
{
  "session_id": "abc123",
  "title": "HNSW discussion",
  "created_at": 1716000000
}
```

```
GET /api/sessions

→ 200 OK
{
  "sessions": [
    {"id": "abc123", "title": "...", "updated_at": ...}
  ]
}
```

```
DELETE /api/sessions/:id

→ 204 No Content
```

### Chat Message

```
POST /api/sessions/:id/messages
Content-Type: application/json

{
  "content": "а какие у него параметры?"
}

→ 202 Accepted
{
  "message_id": "msg456",
  "status": "processing"
}
```

**Стриминг ответа (SSE):**
```
GET /api/stream/session/:id

data: {"type":"token","data":"HNSW"}
data: {"type":"token","data":" имеет"}
data: {"type":"sources","data":[{"ref":1,"title":"..."}]}
event: done
data: {}
```

### Documents

```
GET /api/documents

→ 200 OK
{
  "documents": [
    {
      "id": "doc123",
      "title": "Как масштабировать бизнес",
      "source": "https://youtube.com/...",
      "type": "video",
      "chunk_count": 15,
      "indexed_at": 1716000000
    }
  ]
}
```

```
DELETE /api/documents/:id

→ 204 No Content
```

### Memory

```
POST /api/memory
Content-Type: application/json

{
  "content": "Пользователь интересуется HNSW",
  "category": "fact"
}

→ 201 Created
```

```
GET /api/memory?query=HNSW&top_k=5

→ 200 OK
{
  "memories": [
    {"id": "...", "content": "...", "category": "fact"}
  ]
}
```

### Health

```
GET /health

→ 200 OK
{
  "status": "ok",
  "version": "0.1.0",
  "qdrant": "connected",
  "ollama": "connected"
}
```

## SSE формат

Каждое событие:
```
event: <тип>
data: <JSON>

```

Типы событий:

| Event | Data | Когда |
|-------|------|-------|
| `token` | `{"token":"..."}` | Каждый токен LLM |
| `sources` | `[{"ref":1,"title":"..."}]` | Источники ответа |
| `completed` | `{}` | Ответ завершён |
| `error` | `{"message":"..."}` | Ошибка |

## Конфиг сервера

```yaml
server:
  host: localhost
  port: 8542
  cors:
    allowed_origins: ["*"]
  tls:
    enabled: false
    cert: ""
    key: ""
```

## CORS

Для GUI/Web клиентов:

```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if r.Method == "OPTIONS" {
            w.WriteHeader(204)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

## Go реализация

```go
func (s *Server) Run(ctx context.Context) error {
    mux := http.NewServeMux()

    mux.HandleFunc("POST /api/process", s.HandleProcess)
    mux.HandleFunc("POST /api/query", s.HandleQuery)
    mux.HandleFunc("POST /api/sessions", s.HandleCreateSession)
    mux.HandleFunc("GET /api/sessions", s.HandleListSessions)
    mux.HandleFunc("DELETE /api/sessions/{id}", s.HandleDeleteSession)
    mux.HandleFunc("POST /api/sessions/{id}/messages", s.HandleChatMessage)
    mux.HandleFunc("GET /api/stream/{id}", s.HandleStream)
    mux.HandleFunc("GET /api/documents", s.HandleListDocuments)
    mux.HandleFunc("DELETE /api/documents/{id}", s.HandleDeleteDocument)
    mux.HandleFunc("GET /health", s.HandleHealth)

    addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
    srv := &http.Server{
        Addr:    addr,
        Handler: corsMiddleware(mux),
    }

    go func() {
        <-ctx.Done()
        srv.Shutdown(context.Background())
    }()

    log.Printf("Server starting on %s", addr)
    return srv.ListenAndServe()
}
```
