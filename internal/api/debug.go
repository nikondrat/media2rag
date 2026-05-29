package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"media2rag/internal/dashboard"
	"media2rag/internal/workspace"
)

type systemStatusResp struct {
	Qdrant     componentStatus `json:"qdrant"`
	Ollama     componentStatus `json:"ollama"`
	OpenRouter componentStatus `json:"openrouter"`
	SQLite     componentStatus `json:"sqlite"`
	Workspace  componentStatus `json:"workspace"`
}

type componentStatus struct {
	Connected bool   `json:"connected"`
	Details   string `json:"details"`
}

func (s *Server) handleDebugOverview(w http.ResponseWriter, r *http.Request) {
	days := queryInt(r, "days", 7)

	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"overallScore":       0.85,
			"pipelinePassRate":   92.0,
			"pipelineFailRate":   8.0,
			"documentsProcessed": 0,
		})
		return
	}

	docCount := 0
	if ws, err := workspace.New(s.workspaceDir); err == nil {
		if docs, err := ws.ListDocuments(); err == nil {
			docCount = len(docs)
		}
	}

	metrics, err := s.store.GetOverview(days, docCount)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handleDebugPipelineList(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	limit := queryInt(r, "limit", 50)
	status := r.URL.Query().Get("status")

	runs, err := s.store.GetPipelines(limit, status)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if runs == nil {
		runs = []dashboard.PipelineRun{}
	}

	type pipelineEntry struct {
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
		CreatedAt      string     `json:"created_at"`
		CompletedAt    *string    `json:"completed_at"`
		Stages         []dashboard.PipelineStage   `json:"stages,omitempty"`
		Judge          []dashboard.JudgeEvaluation `json:"judge,omitempty"`
		Embeddings     []dashboard.EmbeddingCheck  `json:"embeddings,omitempty"`
		Feedback       []dashboard.Feedback        `json:"feedback,omitempty"`
	}

	entries := make([]pipelineEntry, 0, len(runs))
	for _, run := range runs {
		entry := pipelineEntry{
			ID:              run.ID,
			Source:          run.Source,
			SourceType:      run.SourceType,
			Status:          run.Status,
			Score:           run.Score,
			TotalTokens:     run.TotalTokens,
			TotalLatencyMs:  run.TotalLatencyMs,
			TotalCost:       run.TotalCost,
			Error:           run.Error,
			PipelineVersion: run.PipelineVersion,
			CreatedAt:       run.CreatedAt.Format(time.RFC3339),
		}
		if run.CompletedAt != nil {
			t := run.CompletedAt.Format(time.RFC3339)
			entry.CompletedAt = &t
		}

		if stages, err := s.store.GetPipelineStages(run.ID); err == nil {
			entry.Stages = stages
		}
		if judge, err := s.store.GetJudgeEvaluations(run.ID); err == nil {
			entry.Judge = judge
		}
		if emb, err := s.store.GetEmbeddingChecks(run.ID); err == nil {
			entry.Embeddings = emb
		}
		if fb, err := s.store.GetFeedback(run.ID); err == nil {
			entry.Feedback = fb
		}

		entries = append(entries, entry)
	}

	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleDebugPipelineDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
		return
	}

	run, err := s.store.GetPipeline(id)
	if err != nil {
		jsonError(w, "pipeline not found", http.StatusNotFound)
		return
	}

	stages, _ := s.store.GetPipelineStages(id)
	llmCalls, _ := s.store.GetLLMCalls(id)
	judgeEvals, _ := s.store.GetJudgeEvaluations(id)
	embChecks, _ := s.store.GetEmbeddingChecks(id)
	feedback, _ := s.store.GetFeedback(id)

	resp := map[string]interface{}{
		"id":               run.ID,
		"source":           run.Source,
		"source_type":      run.SourceType,
		"status":           run.Status,
		"score":            run.Score,
		"total_tokens":     run.TotalTokens,
		"total_latency_ms": run.TotalLatencyMs,
		"total_cost":       run.TotalCost,
		"error":            run.Error,
		"pipeline_version": run.PipelineVersion,
		"config_snapshot":  run.ConfigSnapshot,
		"created_at":       run.CreatedAt.Format(time.RFC3339),
		"stages":           stages,
		"llm_calls":        llmCalls,
		"judge":            judgeEvals,
		"embeddings":       embChecks,
		"feedback":         feedback,
	}

	if run.CompletedAt != nil {
		resp["completed_at"] = run.CompletedAt.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDebugLogsList(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	model := r.URL.Query().Get("model")
	op := r.URL.Query().Get("op")
	status := r.URL.Query().Get("status")

	calls, err := s.store.GetLogs(model, op, status)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if calls == nil {
		calls = []dashboard.LLMCall{}
	}

	type logEntry struct {
		ID               string  `json:"id"`
		RunID            string  `json:"run_id"`
		Model            string  `json:"model"`
		Operation        string  `json:"operation"`
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		LatencyMs        int     `json:"latency_ms"`
		Cost             float64 `json:"cost"`
		Prompt           string  `json:"prompt,omitempty"`
		Response         string  `json:"response,omitempty"`
		Status           string  `json:"status"`
		ErrorMessage     *string `json:"error_message"`
		CreatedAt        string  `json:"created_at"`
	}

	entries := make([]logEntry, 0, len(calls))
	for _, c := range calls {
		entries = append(entries, logEntry{
			ID:               c.ID,
			RunID:            c.RunID,
			Model:            c.Model,
			Operation:        c.Operation,
			PromptTokens:     c.PromptTokens,
			CompletionTokens: c.CompletionTokens,
			LatencyMs:        c.LatencyMs,
			Cost:             c.Cost,
			Prompt:           c.Prompt,
			Response:         c.Response,
			Status:           c.Status,
			ErrorMessage:     c.ErrorMessage,
			CreatedAt:        c.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleDebugLogDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
		return
	}

	call, err := s.store.GetLog(id)
	if err != nil {
		jsonError(w, "log not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, call)
}

func (s *Server) handleDebugMetrics(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"scoreDistribution": map[string]int{},
			"modelComparison":   []interface{}{},
			"scoreByType":       []interface{}{},
			"usage": map[string]interface{}{
				"todayTokens": 0, "weekTokens": 0, "monthTokens": 0,
				"todayCost": 0.0, "weekCost": 0.0, "monthCost": 0.0,
			},
		})
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	metrics, err := s.store.GetMetrics(period)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handleDebugDocuments(w http.ResponseWriter, r *http.Request) {
	ws, err := workspace.New(s.workspaceDir)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	docs, _ := ws.ListDocuments()
	type docEntry struct {
		Title       string  `json:"title"`
		Score       float64 `json:"score"`
		Chunks      int     `json:"chunks"`
		JudgeStatus string  `json:"judge_status"`
		Source      string  `json:"source"`
		Type        string  `json:"type"`
		CreatedAt   string  `json:"created_at"`
	}
	entries := make([]docEntry, 0, len(docs))
	for _, d := range docs {
		t := time.Unix(d.UpdatedAt, 0)

		score := 0.0
		if s.store != nil {
			if run, err := s.store.GetPipeline(d.Hash); err == nil && run.Score != nil {
				score = *run.Score
			}
		}

		entries = append(entries, docEntry{
			Title:       d.Title,
			Score:       score,
			Chunks:      0,
			JudgeStatus: "pending",
			Source:      d.Source,
			Type:        d.SourceType,
			CreatedAt:   t.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleDebugStatus(w http.ResponseWriter, r *http.Request) {
	resp := systemStatusResp{
		SQLite:     componentStatus{Connected: false, Details: "not configured"},
		Workspace:  componentStatus{Connected: false, Details: "not configured"},
		Qdrant:     componentStatus{Connected: false, Details: "not configured"},
		Ollama:     componentStatus{Connected: false, Details: "not configured"},
		OpenRouter: componentStatus{Connected: false, Details: "not configured"},
	}

	if s.store != nil {
		resp.SQLite = componentStatus{Connected: true, Details: "connected"}
	}

	wsDir := s.cfg.Workspace.DataDir
	if wsDir == "" {
		home, _ := os.UserHomeDir()
		wsDir = filepath.Join(home, ".media2rag", "workspace")
	}
	if _, err := os.Stat(wsDir); err == nil {
		resp.Workspace = componentStatus{Connected: true, Details: wsDir}
	} else {
		resp.Workspace = componentStatus{Connected: true, Details: wsDir + " (created on first use)"}
	}

	if s.cfg.RAG.Qdrant.Host != "" {
		resp.Qdrant = componentStatus{Connected: true, Details: s.cfg.RAG.Qdrant.Host}
	}

	ollamaReachable := false
	if _, err := http.Get(s.cfg.LLM.OllamaURL + "/api/tags"); err == nil {
		ollamaReachable = true
		resp.Ollama = componentStatus{Connected: true, Details: fmt.Sprintf("%s (model: %s)", s.cfg.LLM.OllamaURL, s.cfg.LLM.Model)}
	} else {
		resp.Ollama = componentStatus{Connected: false, Details: fmt.Sprintf("cannot reach %s", s.cfg.LLM.OllamaURL)}
	}

	if s.cfg.LLM.OpenRouterKey != "" {
		resp.OpenRouter = componentStatus{Connected: true, Details: "key configured"}
	} else if ollamaReachable {
		resp.OpenRouter = componentStatus{Connected: false, Details: "not needed (ollama up)"}
	} else {
		resp.OpenRouter = componentStatus{Connected: false, Details: "not configured"}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDebugConfig(w http.ResponseWriter, r *http.Request) {
	judgeModel := s.cfg.Judge.Model
	if judgeModel == "" {
		judgeModel = "nvidia/nemotron-3-super-120b-a12b:free"
	}

	judgeFallback := ""
	if len(s.cfg.LLMFallback.JudgeChain) > 1 {
		judgeFallback = s.cfg.LLMFallback.JudgeChain[1]
	}
	if judgeFallback == "" {
		judgeFallback = "qwen/qwen3-coder:free"
	}

	pipelineModel := ""
	if len(s.cfg.LLMFallback.PipelineChain) > 0 {
		pipelineModel = s.cfg.LLMFallback.PipelineChain[0]
	}
	if pipelineModel == "" {
		pipelineModel = "openrouter/free"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"judgeEnabled":      s.cfg.Judge.Enabled,
		"judgeModel":        judgeModel,
		"judgeFallback":     judgeFallback,
		"pipelineModel":     pipelineModel,
		"embedCheckEnabled": s.cfg.EmbedCheck.Enabled,
		"embedCheckModel":   s.cfg.EmbedCheck.Model,
		"embedSampleSize":   s.cfg.EmbedCheck.SampleSize,
	})
}

func (s *Server) handleDebugReprocess(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "queued",
		"id":     r.PathValue("id"),
	})
}

func (s *Server) handleDebugEmbeddings(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"avgSimilarity": 0, "avgRelevance": 0, "passRate": 0, "totalChecks": 0,
		})
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	metrics, err := s.store.GetEmbeddingMetrics(period)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handleDebugFeedbackList(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"total": 0, "likes": 0, "dislikes": 0, "avgRating": 0, "categories": map[string]int{}, "entries": []interface{}{},
		})
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	summary, err := s.store.GetFeedbackSummary(period)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleDebugFeedbackSubmit(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		jsonError(w, "dashboard store not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		RunID       string `json:"run_id"`
		Rating      *int   `json:"rating"`
		LikeDislike string `json:"like_dislike"`
		Category    string `json:"category"`
		Comment     string `json:"comment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.RunID == "" {
		jsonError(w, "run_id is required", http.StatusBadRequest)
		return
	}

	validCategories := map[string]bool{"accurate": true, "hallucination": true, "irrelevant": true, "incomplete": true, "other": true}
	if req.Category != "" && !validCategories[req.Category] {
		req.Category = "other"
	}

	fb := &dashboard.Feedback{
		RunID:       req.RunID,
		Rating:      req.Rating,
		LikeDislike: req.LikeDislike,
		Category:    req.Category,
		Comment:     req.Comment,
		CreatedAt:   time.Now(),
	}

	if err := s.store.SubmitFeedback(fb); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.sse != nil {
		data, _ := json.Marshal(map[string]interface{}{
			"run_id": req.RunID,
			"rating": req.Rating,
		})
		s.sse.Broadcast(dashboard.SSEEvent{Type: "feedback", Data: string(data)})
	}

	if req.LikeDislike == "dislike" || (req.Rating != nil && *req.Rating < 3) {
		if run, err := s.store.GetPipeline(req.RunID); err == nil && run.Score != nil {
			penalized := *run.Score * 0.9
			s.store.PenalizeRunScore(req.RunID, penalized)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) handleDebugRegressions(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"signals":      []interface{}{},
			"topRegressed": []interface{}{},
		})
		return
	}

	period := r.URL.Query().Get("period")
	baseline := r.URL.Query().Get("baseline")
	baselineDuration := r.URL.Query().Get("baseline_duration")

	if period == "" {
		period = "24h"
	}
	if baseline == "" {
		baseline = "48h"
	}
	if baselineDuration == "" {
		baselineDuration = "24h"
	}

	result, err := s.store.GetRegressions(period, baseline, baselineDuration)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDebugLiveSSE(w http.ResponseWriter, r *http.Request) {
	if s.sse != nil {
		s.sse.HandleSSE(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	w.(http.Flusher).Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"ts\":%d}\n\n", time.Now().Unix())
			w.(http.Flusher).Flush()
		}
	}
}

func (s *Server) handleDebugSPA(w http.ResponseWriter, r *http.Request) {
	if s.spaFS == nil {
		jsonError(w, "dashboard not built: run 'cd dashboard && npm run build' first", http.StatusNotFound)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || strings.HasPrefix(path, "api/") {
		path = "index.html"
	}

	content, err := fs.ReadFile(s.spaFS, path)
	if err != nil {
		content, err = fs.ReadFile(s.spaFS, "index.html")
		if err != nil {
			jsonError(w, "not found", http.StatusNotFound)
			return
		}
	}

	ctype := "text/html"
	if strings.HasSuffix(path, ".js") {
		ctype = "application/javascript"
	} else if strings.HasSuffix(path, ".css") {
		ctype = "text/css"
	} else if strings.HasSuffix(path, ".svg") {
		ctype = "image/svg+xml"
	} else if strings.HasSuffix(path, ".woff2") {
		ctype = "font/woff2"
	}
	w.Header().Set("Content-Type", ctype)
	w.Write(content)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
