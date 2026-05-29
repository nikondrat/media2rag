package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"media2rag/internal/store"
)

type Reranker struct {
	baseURL string
	model   string
	client  *http.Client
	enabled bool
}

func NewReranker(baseURL, model string, enabled bool) *Reranker {
	return &Reranker{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{},
		enabled: enabled,
	}
}

func (r *Reranker) IsEnabled() bool {
	return r.enabled
}

type ollamaRerankRequest struct {
	Model    string   `json:"model"`
	Query    string   `json:"query"`
	Documents []string `json:"documents"`
}

type ollamaRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

func (r *Reranker) Rerank(ctx context.Context, query string, results []store.SearchResult, topK int) ([]store.SearchResult, error) {
	if !r.enabled {
		return TopK(results, topK), nil
	}

	documents := make([]string, len(results))
	for i, res := range results {
		documents[i] = res.Payload["content"]
	}

	body := ollamaRerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return TopK(results, topK), fmt.Errorf("encode rerank: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/api/rerank", &buf)
	if err != nil {
		return TopK(results, topK), fmt.Errorf("rerank request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return TopK(results, topK), fmt.Errorf("rerank call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TopK(results, topK), nil
	}

	var rerankResp ollamaRerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		return TopK(results, topK), nil
	}

	reranked := make([]store.SearchResult, 0, len(rerankResp.Results))
	for _, rr := range rerankResp.Results {
		if rr.Index >= 0 && rr.Index < len(results) {
			results[rr.Index].Score = rr.RelevanceScore
			reranked = append(reranked, results[rr.Index])
		}
	}

	for i := 0; i < len(reranked); i++ {
		for j := i + 1; j < len(reranked); j++ {
			if reranked[j].Score > reranked[i].Score {
				reranked[i], reranked[j] = reranked[j], reranked[i]
			}
		}
	}

	return TopK(reranked, topK), nil
}
