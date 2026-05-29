package rag

import (
	"context"
	"fmt"

	"media2rag/internal/llm"
	"media2rag/internal/store"
)

type Engine struct {
	store       store.VectorStore
	llm         llm.LLMClient
	embedClient llm.LLMClient
	embedModel  string
	indexer     *Indexer
	rewriter    *Rewriter
	searcher    *Searcher
	reranker    *Reranker
	parent      *ParentLookup
	context     *ContextBuilder
}

type EngineConfig struct {
	Store          store.VectorStore
	LLM            llm.LLMClient
	EmbedClient    llm.LLMClient
	OllamaURL      string
	EmbedModel     string
	RerankModel    string
	RerankEnabled  bool
}

func NewEngine(cfg EngineConfig) *Engine {
	embedClient := cfg.EmbedClient
	if embedClient == nil {
		embedClient = cfg.LLM
	}
	return &Engine{
		store:       cfg.Store,
		llm:         cfg.LLM,
		embedClient: embedClient,
		embedModel:  cfg.EmbedModel,
		indexer:     NewIndexer(cfg.Store, embedClient),
		rewriter:    NewRewriter(cfg.LLM),
		searcher:    NewSearcher(cfg.Store),
		reranker:    NewReranker(cfg.OllamaURL, cfg.RerankModel, cfg.RerankEnabled),
		parent:      NewParentLookup(cfg.Store),
		context:     NewContextBuilder(cfg.LLM),
	}
}

func (e *Engine) Indexer() *Indexer {
	return e.indexer
}

func (e *Engine) Query(ctx context.Context, q RAGQuery) (*RAGResponse, error) {
	format := e.rewriter.DetectFormat(q.Query)
	rewritten, err := e.rewriter.Rewrite(ctx, q.Query, format)
	if err != nil {
		rewritten = q.Query
	}

	expanded, err := e.rewriter.Expand(ctx, rewritten)
	if err != nil {
		expanded = []string{rewritten}
	}

	allQueries := append([]string{rewritten}, expanded...)

	topK := q.TopK
	if topK <= 0 {
		topK = 5
	}

	var allResults []store.SearchResult
	seen := map[string]bool{}

	for _, qStr := range allQueries {
		embedding, err := e.embedClient.Embed(ctx, qStr)
		if err != nil {
			continue
		}

		results, err := e.searcher.HybridSearch(ctx, qStr, embedding, topK*2)
		if err != nil {
			continue
		}

		for _, r := range results {
			if !seen[r.ID] {
				seen[r.ID] = true
				allResults = append(allResults, r)
			}
		}
	}

	reranked, err := e.reranker.Rerank(ctx, rewritten, allResults, topK)
	if err != nil {
		reranked = TopK(allResults, topK)
	}

	parentResults, err := e.parent.Lookup(ctx, reranked)
	if err != nil {
		parentResults = reranked
	}

	resp, err := e.context.BuildAndQuery(ctx, q.Query, parentResults)
	if err != nil {
		return nil, fmt.Errorf("rag query: %w", err)
	}

	sources := make([]Source, len(parentResults))
	for i, r := range parentResults {
		sources[i] = Source{
			Index:   i + 1,
			Title:   r.Payload["document_id"],
			Type:    r.Payload["chunk_type"],
			Source:  r.Payload["document_id"],
			Content: r.Payload["content"],
		}
	}

	return &RAGResponse{
		Answer:  resp.Message.Content,
		Sources: sources,
	}, nil
}
