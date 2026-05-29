package dashboard

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"media2rag/internal/model"
	"media2rag/internal/pipeline"
)

type Tracer struct {
	store *Store
	sse   *SSEBroadcaster
}

func NewTracer(store *Store, sse *SSEBroadcaster) *Tracer {
	return &Tracer{store: store, sse: sse}
}

func GenerateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (t *Tracer) SaveStage(entry pipeline.TraceEntry) {
	stage := &PipelineStage{
		RunID:     entry.RunID,
		Name:      entry.StageName,
		Seq:       entry.Seq,
		Status:    "completed",
		Score:     float64Ptr(entry.Score),
		LatencyMs: entry.LatencyMs,
		TokensIn:  entry.TokensIn,
		TokensOut: entry.TokensOut,
		Prompt:    entry.Prompt,
		Response:  entry.Response,
		Model:     entry.Model,
		CreatedAt: time.Now(),
	}
	if entry.Error != "" {
		stage.Status = "failed"
		e := entry.Error
		stage.Error = &e
	}

	t.store.db.Exec("PRAGMA busy_timeout=5000")
	t.store.db.Exec(`
		INSERT INTO pipeline_stages (run_id, name, seq, status, score, latency_ms, tokens_in, tokens_out, prompt, response, model, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, stage.RunID, stage.Name, stage.Seq, stage.Status, stage.Score, stage.LatencyMs,
		stage.TokensIn, stage.TokensOut, stage.Prompt, stage.Response, stage.Model,
		stage.Error, stage.CreatedAt.Format(time.RFC3339))
}

func (t *Tracer) SaveLLMCall(runID, model, operation string, promptTokens, completionTokens, latencyMs int, cost float64, prompt, response, status, errMsg string) {
	call := &LLMCall{
		ID:              GenerateID(),
		RunID:           runID,
		Model:           model,
		Operation:       operation,
		PromptTokens:    promptTokens,
		CompletionTokens: completionTokens,
		LatencyMs:       latencyMs,
		Cost:            cost,
		Prompt:          prompt,
		Response:        response,
		Status:          status,
		CreatedAt:       time.Now(),
	}
	if errMsg != "" {
		call.ErrorMessage = &errMsg
	}

	t.store.db.Exec("PRAGMA busy_timeout=5000")
	t.store.db.Exec(`
		INSERT INTO llm_calls (id, run_id, model, operation, prompt_tokens, completion_tokens, latency_ms, cost, prompt, response, status, error_message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, call.ID, call.RunID, call.Model, call.Operation, call.PromptTokens,
		call.CompletionTokens, call.LatencyMs, call.Cost, call.Prompt, call.Response,
		call.Status, call.ErrorMessage, call.CreatedAt.Format(time.RFC3339))

	t.llmCallSSE(call)
}

func (t *Tracer) SaveRunComplete(runID string, score float64, totalTokens, totalLatencyMs int, totalCost float64, err string) {
	now := time.Now()
	e := ""
	if err != "" {
		e = err
	}
	t.store.db.Exec("PRAGMA busy_timeout=5000")
	t.store.db.Exec(`
		UPDATE pipeline_runs SET status='completed', score=?, total_tokens=?, total_latency_ms=?, total_cost=?, error=?, completed_at=? WHERE id=?
	`, score, totalTokens, totalLatencyMs, totalCost, e, now.Format(time.RFC3339), runID)
}

func (t *Tracer) SaveRunStart(runID, source, sourceType string) {
	run := &PipelineRun{
		ID:         runID,
		Source:     source,
		SourceType: sourceType,
		Status:     "running",
		CreatedAt:  time.Now(),
	}
	t.store.db.Exec("PRAGMA busy_timeout=5000")
	t.store.db.Exec(`
		INSERT INTO pipeline_runs (id, source, source_type, status, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, run.ID, run.Source, run.SourceType, run.Status, run.CreatedAt.Format(time.RFC3339))
}

func (t *Tracer) BroadcastEvent(typ string, data map[string]interface{}) {
	if t.sse != nil {
		d, _ := json.Marshal(data)
		t.sse.Broadcast(SSEEvent{Type: typ, Data: string(d)})
	}
}

func (t *Tracer) llmCallSSE(call *LLMCall) {
	if t.sse != nil {
		data, _ := json.Marshal(map[string]interface{}{
			"call_id": call.ID, "model": call.Model, "operation": call.Operation,
			"tokens": call.PromptTokens + call.CompletionTokens, "latency": call.LatencyMs,
		})
		t.sse.Broadcast(SSEEvent{Type: "llm_call", Data: string(data)})
	}
}

func (t *Tracer) SaveRun(run *PipelineRun) error {
	t.store.db.Exec("PRAGMA busy_timeout=5000")

	_, err := t.store.db.Exec(`
		INSERT INTO pipeline_runs (id, source, source_type, status, score, total_tokens, total_latency_ms, total_cost, error, pipeline_version, config_snapshot, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, score=excluded.score, total_tokens=excluded.total_tokens,
			total_latency_ms=excluded.total_latency_ms, total_cost=excluded.total_cost,
			error=excluded.error, completed_at=excluded.completed_at
	`,
		run.ID, run.Source, run.SourceType, run.Status, run.Score, run.TotalTokens,
		run.TotalLatencyMs, run.TotalCost, run.Error, run.PipelineVersion, run.ConfigSnapshot,
		run.CreatedAt.Format(time.RFC3339), nullTime(run.CompletedAt))

	if err != nil {
		return fmt.Errorf("save run: %w", err)
	}

	return nil
}

func (t *Tracer) SaveJudgeEvaluation(eval *JudgeEvaluation) error {
	_, err := t.store.db.Exec(`
		INSERT INTO judge_evaluations (run_id, judge_model, judge_type, prompt, response, score, reasoning, passed, latency_ms, tokens_used, cost, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		eval.RunID, eval.JudgeModel, eval.JudgeType, eval.Prompt, eval.Response,
		eval.Score, eval.Reasoning, boolToInt(eval.Passed), eval.LatencyMs,
		eval.TokensUsed, eval.Cost, eval.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("save judge: %w", err)
	}

	return nil
}

func (t *Tracer) SaveEmbeddingCheck(check *EmbeddingCheck) error {
	_, err := t.store.db.Exec(`
		INSERT INTO embedding_checks (run_id, query_text, chunk_text, similarity_score, relevance_score, passed, latency_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		check.RunID, check.QueryText, check.ChunkText, check.SimilarityScore,
		check.RelevanceScore, boolToInt(check.Passed), check.LatencyMs,
		check.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("save embedding check: %w", err)
	}

	return nil
}

func (t *Tracer) SaveFeedback(fb *Feedback) error {
	_, err := t.store.db.Exec(`
		INSERT INTO feedback (run_id, rating, like_dislike, category, comment, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		fb.RunID, fb.Rating, fb.LikeDislike, fb.Category, fb.Comment,
		fb.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("save feedback: %w", err)
	}

	return nil
}

func (t *Tracer) SavePromptVersion(pv *PromptVersion) error {
	_, err := t.store.db.Exec(`
		INSERT INTO prompt_versions (name, version, prompt_text, model, parameters, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name, version) DO NOTHING
	`,
		pv.Name, pv.Version, pv.PromptText, pv.Model, pv.Parameters,
		pv.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("save prompt version: %w", err)
	}

	return nil
}

func (t *Tracer) GetOrCreatePromptVersion(name, promptText, model, params string) (int, error) {
	var version int
	err := t.store.db.QueryRow(
		`SELECT version FROM prompt_versions WHERE name = ? AND prompt_text = ?`, name, promptText,
	).Scan(&version)
	if err == nil {
		return version, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("query prompt version: %w", err)
	}

	err = t.store.db.QueryRow(
		`SELECT COALESCE(MAX(version), 0) + 1 FROM prompt_versions WHERE name = ?`, name,
	).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("next version: %w", err)
	}

	pv := &PromptVersion{
		Name:       name,
		Version:    version,
		PromptText: promptText,
		Model:      model,
		Parameters: params,
		CreatedAt:  time.Now(),
	}
	if err := t.SavePromptVersion(pv); err != nil {
		return 0, err
	}

	return version, nil
}

func (t *Tracer) Broadcast(event model.Event) {
	if t.sse != nil {
		data, _ := json.Marshal(event.Data)
		t.sse.Broadcast(SSEEvent{Type: event.Type, Data: string(data)})
	}
}

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func float64Ptr(f float64) *float64 {
	return &f
}
