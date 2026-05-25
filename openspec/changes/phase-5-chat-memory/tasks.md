## 1. SQLite Store (`internal/store/sqlite.go`)

- [ ] 1.1 Initialize SQLite database with `modernc.org/sqlite` (pure Go)
- [ ] 1.2 Create schema: sessions, messages, memory_facts tables
- [ ] 1.3 Enable WAL mode for concurrency
- [ ] 1.4 Implement session CRUD: Create, Get, List, Delete
- [ ] 1.5 Implement message storage: AddMessage, GetMessages

## 2. Chat Session (`internal/chat/session.go`)

- [ ] 2.1 Implement `Session` struct with ID, title, history
- [ ] 2.2 Implement `NewSession()` — create with empty history
- [ ] 2.3 Implement `LoadSession(id)` — load from SQLite
- [ ] 2.4 Implement `AddMessage()` — store message, update session
- [ ] 2.5 Implement `GetHistory(limit)` — retrieve recent messages

## 3. Memory Store (`internal/memory/store.go`)

- [ ] 3.1 Implement `Store(userID, content, category)` — embed + save to Qdrant
- [ ] 3.2 Implement `Recall(userID, query, topK)` — semantic search
- [ ] 3.3 Implement `Delete(entryID)` — remove from Qdrant
- [ ] 3.4 Implement `List(userID)` — list all facts

## 4. Context Builder (`internal/chat/context.go`)

- [ ] 4.1 Implement context assembly: history + memories + RAG sources
- [ ] 4.2 Implement section formatting with `--- Section ---` headers
- [ ] 4.3 Implement summarization of old messages via LLM

## 5. Chat Command (`cmd/media2rag/chat.go`)

- [ ] 5.1 Implement interactive readline loop
- [ ] 5.2 Implement session creation/loading
- [ ] 5.3 Implement LLM streaming to terminal
- [ ] 5.4 Implement source display after response
- [ ] 5.5 Implement `--session` flag for continuing sessions
- [ ] 5.6 Implement fact extraction after each response

## 6. Integration

- [ ] 6.1 Wire SQLite store into root command init
- [ ] 6.2 Wire memory store into RAG engine
- [ ] 6.3 `./media2rag chat` starts interactive session
- [ ] 6.4 Chat remembers context between messages
- [ ] 6.5 Facts are extracted and recalled
