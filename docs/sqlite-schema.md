# SQLite Schema — media2rag

## Database Location

`~/.media2rag/media2rag.db`

---

## Tables

### sessions

Chat sessions.

```sql
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,  -- UUID
    title       TEXT NOT NULL,
    skill       TEXT,              -- active skill name (nullable)
    created_at  INTEGER NOT NULL,  -- unix timestamp
    updated_at  INTEGER NOT NULL
);

CREATE INDEX idx_sessions_updated ON sessions(updated_at DESC);
```

### messages

Chat messages within a session.

```sql
CREATE TABLE messages (
    id          TEXT PRIMARY KEY,  -- UUID
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'system')),
    content     TEXT NOT NULL,
    sources     TEXT,              -- JSON array of sources (for assistant)
    created_at  INTEGER NOT NULL
);

CREATE INDEX idx_messages_session ON messages(session_id, created_at);
```

### memories

Extracted facts and user information.

```sql
CREATE TABLE memories (
    id          TEXT PRIMARY KEY,  -- UUID
    user_id     TEXT NOT NULL DEFAULT 'default',
    content     TEXT NOT NULL,
    category    TEXT NOT NULL CHECK(category IN ('fact', 'preference', 'goal', 'weakness', 'strength', 'pattern')),
    source      TEXT,              -- message_id or document_id that generated this
    created_at  INTEGER NOT NULL
);

CREATE INDEX idx_memories_user ON memories(user_id);
CREATE INDEX idx_memories_category ON memories(user_id, category);
```

### documents

Processed document metadata (mirrors workspace + Qdrant).

```sql
CREATE TABLE documents (
    id          TEXT PRIMARY KEY,  -- source hash
    title       TEXT NOT NULL,
    source      TEXT NOT NULL,
    doc_type    TEXT NOT NULL,     -- video, audio, markdown, url, pdf
    language    TEXT,
    word_count  INTEGER,
    skill       TEXT,              -- skill used for processing (nullable)
    status      TEXT NOT NULL DEFAULT 'processing' CHECK(status IN ('processing', 'completed', 'failed')),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_skill ON documents(skill);
```

### document_versions

Version history for reprocessed documents.

```sql
CREATE TABLE document_versions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    backend     TEXT NOT NULL,     -- ollama, openrouter
    model       TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    UNIQUE(document_id, version)
);
```

### skills

Installed skills registry.

```sql
CREATE TABLE skills (
    name        TEXT PRIMARY KEY,
    version     TEXT NOT NULL,
    description TEXT,
    enabled     INTEGER NOT NULL DEFAULT 0,
    installed_at INTEGER NOT NULL
);
```

### usage_stats

LLM usage tracking for cost control.

```sql
CREATE TABLE usage_stats (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    operation   TEXT NOT NULL,     -- process, chat, query, coach
    model       TEXT NOT NULL,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    cost_usd    REAL,
    created_at  INTEGER NOT NULL
);

CREATE INDEX idx_usage_stats_date ON usage_stats(created_at);
```

---

## Migrations

Migration files stored in `migrations/` directory:

```
migrations/
├── 001_initial.sql
├── 002_add_skills.sql
├── 003_add_usage_stats.sql
└── ...
```

Applied on startup using a simple migration runner:

```go
type Migrator struct {
    db         *sql.DB
    migrationsDir string
}

func (m *Migrator) Migrate() error {
    // Read migration files in order
    // Track applied migrations in a schema_migrations table
    // Apply pending migrations
}
```

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at INTEGER NOT NULL
);
```

---

## Queries

### Session operations

```sql
-- Create session
INSERT INTO sessions (id, title, skill, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- Get session with recent messages
SELECT s.*, m.id as msg_id, m.role, m.content, m.sources, m.created_at as msg_created
FROM sessions s
LEFT JOIN messages m ON s.id = m.session_id
WHERE s.id = ?
ORDER BY m.created_at DESC
LIMIT 20;

-- Update session timestamp
UPDATE sessions SET updated_at = ? WHERE id = ?;

-- Delete session (cascade deletes messages)
DELETE FROM sessions WHERE id = ?;
```

### Memory operations

```sql
-- Store memory
INSERT INTO memories (id, user_id, content, category, source, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- Search memories (full-text via FTS or semantic via Qdrant)
SELECT * FROM memories
WHERE user_id = ? AND category IN (?, ?, ?)
ORDER BY created_at DESC
LIMIT ?;

-- Delete memory
DELETE FROM memories WHERE id = ? AND user_id = ?;
```

### Usage tracking

```sql
-- Record usage
INSERT INTO usage_stats (operation, model, prompt_tokens, completion_tokens, cost_usd, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- Get usage summary (last 30 days)
SELECT operation, model, SUM(prompt_tokens + completion_tokens) as total_tokens, SUM(cost_usd) as total_cost
FROM usage_stats
WHERE created_at > ?
GROUP BY operation, model;
```

---

## Connection Config

```yaml
sqlite:
  path: ~/.media2rag/media2rag.db
  max_connections: 5
  busy_timeout: 5000ms
  journal_mode: WAL
  synchronous: NORMAL
```
