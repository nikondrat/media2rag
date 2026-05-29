package dashboard

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type PipelineRun struct {
	ID             string     `json:"id"`
	Source         string     `json:"source"`
	SourceType     string     `json:"source_type"`
	Status         string     `json:"status"`
	Score          *float64   `json:"score"`
	TotalTokens    int        `json:"total_tokens"`
	TotalLatencyMs int        `json:"total_latency_ms"`
	TotalCost      float64    `json:"total_cost"`
	Error          *string    `json:"error"`
	PipelineVersion string    `json:"pipeline_version"`
	ConfigSnapshot string     `json:"config_snapshot"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

type PipelineStage struct {
	ID          int        `json:"id"`
	RunID       string     `json:"run_id"`
	Name        string     `json:"name"`
	Seq         int        `json:"seq"`
	Status      string     `json:"status"`
	Score       *float64   `json:"score"`
	LatencyMs   int        `json:"latency_ms"`
	TokensIn    int        `json:"tokens_in"`
	TokensOut   int        `json:"tokens_out"`
	Prompt      string     `json:"prompt,omitempty"`
	Response    string     `json:"response,omitempty"`
	Model       string     `json:"model"`
	Error       *string    `json:"error"`
	CreatedAt   time.Time  `json:"created_at"`
}

type LLMCall struct {
	ID              string    `json:"id"`
	RunID           string    `json:"run_id"`
	Model           string    `json:"model"`
	Operation       string    `json:"operation"`
	PromptTokens    int       `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	LatencyMs       int       `json:"latency_ms"`
	Cost            float64   `json:"cost"`
	Prompt          string    `json:"prompt,omitempty"`
	Response        string    `json:"response,omitempty"`
	Status          string    `json:"status"`
	ErrorMessage    *string   `json:"error_message"`
	CreatedAt       time.Time `json:"created_at"`
}

type JudgeEvaluation struct {
	ID          int        `json:"id"`
	RunID       string     `json:"run_id"`
	JudgeModel  string     `json:"judge_model"`
	JudgeType   string     `json:"judge_type"`
	Prompt      string     `json:"prompt"`
	Response    string     `json:"response"`
	Score       float64    `json:"score"`
	Reasoning   string     `json:"reasoning"`
	Passed      bool       `json:"passed"`
	LatencyMs   int        `json:"latency_ms"`
	TokensUsed  int        `json:"tokens_used"`
	Cost        float64    `json:"cost"`
	CreatedAt   time.Time  `json:"created_at"`
}

type EmbeddingCheck struct {
	ID               int       `json:"id"`
	RunID            string    `json:"run_id"`
	QueryText        string    `json:"query_text"`
	ChunkText        string    `json:"chunk_text"`
	SimilarityScore  float64   `json:"similarity_score"`
	RelevanceScore   float64   `json:"relevance_score"`
	Passed           bool      `json:"passed"`
	LatencyMs        int       `json:"latency_ms"`
	CreatedAt        time.Time `json:"created_at"`
}

type Feedback struct {
	ID          int       `json:"id"`
	RunID       string    `json:"run_id"`
	Rating      *int      `json:"rating"`
	LikeDislike string    `json:"like_dislike"`
	Category    string    `json:"category"`
	Comment     string    `json:"comment"`
	CreatedAt   time.Time `json:"created_at"`
}

type MetricsCache struct {
	CacheKey   string    `json:"cache_key"`
	Data       string    `json:"data"`
	ComputedAt time.Time `json:"computed_at"`
	TTLSeconds int       `json:"ttl_seconds"`
}

type PromptVersion struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Version     int       `json:"version"`
	PromptText  string    `json:"prompt_text"`
	Model       string    `json:"model"`
	Parameters  string    `json:"parameters"`
	CreatedAt   time.Time `json:"created_at"`
}

const schema = `
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'running',
    score REAL,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    total_latency_ms INTEGER NOT NULL DEFAULT 0,
    total_cost REAL NOT NULL DEFAULT 0.0,
    error TEXT,
    pipeline_version TEXT NOT NULL DEFAULT '',
    config_snapshot TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    completed_at TEXT
);

CREATE TABLE IF NOT EXISTS pipeline_stages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES pipeline_runs(id),
    name TEXT NOT NULL,
    seq INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'completed',
    score REAL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    tokens_in INTEGER NOT NULL DEFAULT 0,
    tokens_out INTEGER NOT NULL DEFAULT 0,
    prompt TEXT NOT NULL DEFAULT '',
    response TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    error TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_calls (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES pipeline_runs(id),
    model TEXT NOT NULL,
    operation TEXT NOT NULL,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    cost REAL NOT NULL DEFAULT 0.0,
    prompt TEXT NOT NULL DEFAULT '',
    response TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'success',
    error_message TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS judge_evaluations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES pipeline_runs(id),
    judge_model TEXT NOT NULL,
    judge_type TEXT NOT NULL,
    prompt TEXT NOT NULL DEFAULT '',
    response TEXT NOT NULL DEFAULT '',
    score REAL NOT NULL DEFAULT 0.0,
    reasoning TEXT NOT NULL DEFAULT '',
    passed INTEGER NOT NULL DEFAULT 1,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    tokens_used INTEGER NOT NULL DEFAULT 0,
    cost REAL NOT NULL DEFAULT 0.0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS embedding_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES pipeline_runs(id),
    query_text TEXT NOT NULL DEFAULT '',
    chunk_text TEXT NOT NULL DEFAULT '',
    similarity_score REAL NOT NULL DEFAULT 0.0,
    relevance_score REAL NOT NULL DEFAULT 0.0,
    passed INTEGER NOT NULL DEFAULT 1,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES pipeline_runs(id),
    rating INTEGER,
    like_dislike TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    comment TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS metrics_cache (
    cache_key TEXT PRIMARY KEY,
    data TEXT NOT NULL,
    computed_at TEXT NOT NULL,
    ttl_seconds INTEGER NOT NULL DEFAULT 60
);

CREATE TABLE IF NOT EXISTS prompt_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    version INTEGER NOT NULL,
    prompt_text TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    parameters TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    UNIQUE(name, version)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_stages_run_id ON pipeline_stages(run_id);
CREATE INDEX IF NOT EXISTS idx_llm_calls_run_id ON llm_calls(run_id);
CREATE INDEX IF NOT EXISTS idx_judge_evaluations_run_id ON judge_evaluations(run_id);
CREATE INDEX IF NOT EXISTS idx_embedding_checks_run_id ON embedding_checks(run_id);
CREATE INDEX IF NOT EXISTS idx_feedback_run_id ON feedback(run_id);
CREATE INDEX IF NOT EXISTS idx_llm_calls_created_at ON llm_calls(created_at);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_created_at ON pipeline_runs(created_at);
`

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func truncateText(text string, maxLen int) string {
	if len(text) > maxLen {
		return text[:maxLen] + "...[truncated]"
	}
	return text
}
