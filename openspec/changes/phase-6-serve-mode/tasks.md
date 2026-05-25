## 1. HTTP Server (`internal/api/server.go`)

- [ ] 1.1 Implement `Server` struct with config, router, dependencies
- [ ] 1.2 Implement `Run(ctx)` — start HTTP server with graceful shutdown
- [ ] 1.3 Implement route registration for all endpoints
- [ ] 1.4 Implement SIGTERM handler for graceful shutdown

## 2. REST API Handlers (`internal/api/`)

- [ ] 2.1 Implement `HandleProcess` — POST /api/process, return task_id
- [ ] 2.2 Implement `HandleQuery` — POST /api/query, return answer
- [ ] 2.3 Implement `HandleCreateSession` — POST /api/sessions
- [ ] 2.4 Implement `HandleListSessions` — GET /api/sessions
- [ ] 2.5 Implement `HandleDeleteSession` — DELETE /api/sessions/:id
- [ ] 2.6 Implement `HandleChatMessage` — POST /api/sessions/:id/messages
- [ ] 2.7 Implement `HandleListDocuments` — GET /api/documents
- [ ] 2.8 Implement `HandleDeleteDocument` — DELETE /api/documents/:id
- [ ] 2.9 Implement `HandleAddMemory` — POST /api/memory
- [ ] 2.10 Implement `HandleSearchMemory` — GET /api/memory

## 3. SSE Streaming (`internal/api/stream.go`)

- [ ] 3.1 Implement `HandleStream` — GET /api/stream/{id}
- [ ] 3.2 Implement SSE event formatting: `event: type\ndata: json`
- [ ] 3.3 Implement stream subscription with goroutine per client
- [ ] 3.4 Implement disconnect cleanup
- [ ] 3.5 Wire process events to SSE stream
- [ ] 3.6 Wire LLM streaming tokens to SSE

## 4. Middleware (`internal/api/middleware.go`)

- [ ] 4.1 Implement CORS middleware: Allow-Origin, Allow-Methods, Allow-Headers
- [ ] 4.2 Implement OPTIONS preflight handler
- [ ] 4.3 Implement auth middleware: Bearer token validation
- [ ] 4.4 Implement skip auth when no API key configured

## 5. Health Endpoint (`internal/api/health.go`)

- [ ] 5.1 Implement `HandleHealth` — GET /health
- [ ] 5.2 Implement Qdrant connectivity check
- [ ] 5.3 Implement Ollama connectivity check
- [ ] 5.4 Return version, status, service health

## 6. Serve Command (`cmd/media2rag/serve.go`)

- [ ] 6.1 Update serve command: load config, init dependencies, start server
- [ ] 6.2 Wire all API handlers
- [ ] 6.3 `./media2rag serve` starts HTTP server
- [ ] 6.4 `curl http://localhost:8542/health` returns OK
