package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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

	emitter := events.NewHumanEmitter()
	pipe := pipeline.New(pipeline.DefaultConfig(), s.llmClient)

	ec := model.ExtractedContent{
		Content:  markdown,
		Source:   req.Source,
		DocType:  sourceType,
		WordCount: countWords(markdown),
	}

	ragDoc, err := pipe.Run(r.Context(), ec, emitter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("pipeline: %v", err)})
		return
	}

	version, err := ws.SaveVersion(wDoc.Hash, ragDoc.Markdown)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, processResponse{Error: fmt.Sprintf("save version: %v", err)})
		return
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
