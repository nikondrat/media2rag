package api

import (
	"encoding/json"
	"net/http"

	"media2rag/internal/model"
)

type healthResponse struct {
	Status    string `json:"status"`
	LLM       string `json:"llm"`
	Qdrant    string `json:"qdrant"`
	Workspace string `json:"workspace"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:    "ok",
		LLM:       "unknown",
		Qdrant:    "unknown",
		Workspace: "unknown",
	}

	if _, err := s.llmClient.Chat(r.Context(), model.ChatRequest{
		Messages: []model.Message{{Role: "user", Content: "ping"}},
	}); err == nil {
		resp.LLM = "connected"
	} else {
		resp.LLM = "unavailable"
	}

	if s.cfg.RAG.Qdrant.Host != "" {
		resp.Qdrant = "configured"
	} else {
		resp.Qdrant = "not configured"
	}

	if s.cfg.Workspace.DataDir != "" {
		resp.Workspace = "configured"
	} else {
		resp.Workspace = "not configured"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
