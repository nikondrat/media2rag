package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"media2rag/internal/dashboard"
	"media2rag/internal/embedcheck"
	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/pipeline"
	"media2rag/internal/rag"
	"media2rag/internal/service"
	"media2rag/internal/workspace"
)

type processRequest struct {
	Source string `json:"source"`
}

type processResponse struct {
	Hash    string `json:"hash,omitempty"`
	Title   string `json:"title,omitempty"`
	RunID   string `json:"run_id,omitempty"`
	Version int    `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	var req processRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Source == "" {
		http.Error(w, "source is required", http.StatusBadRequest)
		return
	}

	extractor, err := s.extractorRegistry.Find(req.Source)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, processResponse{Error: fmt.Sprintf("unsupported source: %s", req.Source)})
		return
	}

	markdown, err := extractor.Extract(r.Context(), req.Source)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("extract: %v", err)})
		return
	}

	ws, err := workspace.New(s.workspaceDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("workspace: %v", err)})
		return
	}

	sourceType := workspace.SourceType(req.Source)
	wDoc, err := ws.CreateDocument(req.Source, sourceType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("create document: %v", err)})
		return
	}

	if err := ws.SaveSource(wDoc.Hash, markdown); err != nil {
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("save source: %v", err)})
		return
	}

	runID := dashboard.GenerateID()
	startTime := time.Now()

	if s.tracer != nil {
		s.tracer.SaveRunStart(runID, req.Source, sourceType)
		s.tracer.BroadcastEvent("pipeline_start", map[string]interface{}{
			"run_id": runID, "source": req.Source, "timestamp": startTime.Unix(),
		})
	}

	pcfg := pipeline.DefaultConfig()
	if s.cfg.Pipeline.ChunkSize > 0 {
		pcfg.ChunkSize = s.cfg.Pipeline.ChunkSize
	}
	if s.cfg.Pipeline.ChunkOverlap > 0 {
		pcfg.ChunkOverlap = s.cfg.Pipeline.ChunkOverlap
	}
	pcfg.MaxConcurrency = 3

	pipe := pipeline.New(pcfg, s.llmClient)

	if s.tracer != nil {
		pipe.SetRunID(runID)
		pipe.SetTracer(s.tracer)
	}

	ec := model.ExtractedContent{
		Content:   markdown,
		Source:    req.Source,
		DocType:   sourceType,
		WordCount: countWords(markdown),
	}

	pipeStart := time.Now()
	emitter := events.NewHumanEmitter()
	ragDoc, err := pipe.Run(r.Context(), ec, emitter)
	pipelineLatency := int(time.Since(pipeStart).Milliseconds())

	if err != nil {
		if s.tracer != nil {
			s.tracer.BroadcastEvent("pipeline_complete", map[string]interface{}{
				"run_id": runID, "status": "failed", "error": err.Error(), "latency_ms": pipelineLatency,
			})
		}
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("pipeline: %v", err)})
		return
	}

	version, err := ws.SaveVersion(wDoc.Hash, ragDoc.Markdown)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("save version: %v", err)})
		return
	}

	totalTokens := countWords(ragDoc.Markdown)
	defaultScore := 0.85

	if s.tracer != nil {
		s.tracer.SaveRunComplete(runID, defaultScore, totalTokens, int(time.Since(startTime).Milliseconds()), 0, "")
		s.tracer.BroadcastEvent("pipeline_complete", map[string]interface{}{
			"run_id": runID, "score": defaultScore, "latency_ms": pipelineLatency, "status": "completed",
		})
	}

	if s.judgeRunner != nil && s.cfg.Judge.Enabled {
		go func() {
			ctx := r.Context()
			evals, aggScore := s.judgeRunner.EvaluateAll(ctx, runID, ragDoc.Metadata.Title, ragDoc.Markdown, ragDoc.Markdown)

			if s.tracer != nil {
				s.tracer.BroadcastEvent("judge_complete", map[string]interface{}{
					"run_id": runID, "score": aggScore,
				})
			}
			_ = evals
		}()
	}

	if s.embedChecker != nil && s.cfg.EmbedCheck.Enabled {
		go func() {
			_ = s.embedChecker.Check(r.Context(), runID, ragDoc.Metadata.Title, []embedcheck.SimilarityResult{})
		}()
	}

	embedClient := llm.NewOllamaClient(s.cfg.LLM.OllamaURL, s.cfg.LLM.EmbedModel)
	qdrantSvc := service.NewQdrant(s.cfg.RAG.Qdrant)
	if st, err := qdrantSvc.EnsureRunning(r.Context(), embedClient); err == nil {
		indexer := rag.NewIndexer(st, embedClient)
		_ = indexer.IndexDocument(r.Context(), wDoc.Hash, ragDoc.Markdown)
		qdrantSvc.Stop(r.Context())
	}

	writeJSON(w, http.StatusOK, processResponse{
		Hash:    wDoc.Hash,
		Title:   ragDoc.Metadata.Title,
		RunID:   runID,
		Version: version,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func countWords(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	inWord := false
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' {
			inWord = false
		} else {
			if !inWord {
				count++
				inWord = true
			}
		}
	}
	return count - 1
}
