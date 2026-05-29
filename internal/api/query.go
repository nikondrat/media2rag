package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"media2rag/internal/llm"
	"media2rag/internal/rag"
	"media2rag/internal/service"
)

type queryRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"`
}

type queryResponse struct {
	Answer  string       `json:"answer"`
	Sources []rag.Source `json:"sources,omitempty"`
	Error   string       `json:"error,omitempty"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	if req.TopK <= 0 {
		req.TopK = 5
	}

	embedClient := llm.NewOllamaClient(s.cfg.LLM.OllamaURL, s.cfg.LLM.EmbedModel)
	qdrantSvc := service.NewQdrant(s.cfg.RAG.Qdrant)
	st, err := qdrantSvc.EnsureRunning(r.Context(), embedClient)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, queryResponse{Error: fmt.Sprintf("qdrant: %v", err)})
		return
	}
	defer qdrantSvc.Stop(r.Context())

	engine := rag.NewEngine(rag.EngineConfig{
		Store:         st,
		LLM:           s.llmClient,
		EmbedClient:   embedClient,
		OllamaURL:     s.cfg.LLM.OllamaURL,
		EmbedModel:    s.cfg.LLM.EmbedModel,
		RerankModel:   s.cfg.RAG.RerankModel,
		RerankEnabled: s.cfg.RAG.Rerank,
	})

	resp, err := engine.Query(r.Context(), rag.RAGQuery{
		Query: req.Query,
		TopK:  req.TopK,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, queryResponse{Error: fmt.Sprintf("rag query: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, queryResponse{
		Answer:  resp.Answer,
		Sources: resp.Sources,
	})
}
