package rag

import (
	"context"

	"media2rag/internal/store"
)

type Searcher struct {
	st store.VectorStore
}

func NewSearcher(st store.VectorStore) *Searcher {
	return &Searcher{st: st}
}

func (s *Searcher) SearchDense(ctx context.Context, query []float32, topK uint64) ([]store.SearchResult, error) {
	return s.st.SearchPoints(ctx, "documents", query, topK)
}

func (s *Searcher) SearchSparse(ctx context.Context, query string, denseResults []store.SearchResult) []store.SearchResult {
	return KeywordOverlapSearch(query, denseResults)
}

func (s *Searcher) HybridSearch(ctx context.Context, query string, embedding []float32, topK int) ([]store.SearchResult, error) {
	denseLimit := uint64(topK * 2)
	denseResults, err := s.SearchDense(ctx, embedding, denseLimit)
	if err != nil {
		return nil, err
	}

	sparseResults := KeywordOverlapSearch(query, denseResults)
	if len(sparseResults) == 0 {
		sparseResults = denseResults
	}

	fused := RRF(denseResults, sparseResults, 60.0)
	return TopK(fused, topK), nil
}
