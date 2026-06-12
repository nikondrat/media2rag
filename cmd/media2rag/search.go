package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/graph"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/qdrant"
)

var (
	searchQdrantURL       string
	searchQdrantAPIKey    string
	searchCollection      string
	searchTop             int
	searchMinScore        float64
	searchGraphPath       string
	searchCommunitiesPath string
	searchDepth           int
	searchFormat          string
	searchBackend         string
	searchModel           string
	searchEmbedBackend    string
	searchEmbedModel      string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Hybrid RAG+GraphRAG search",
	Long: `Hybrid search combining semantic RAG with knowledge graph traversal.
RAG finds relevant chunks, extracts entity names, then GraphRAG traverses causal chains.

Usage:
  media2rag search "хочу масштаб"
  media2rag search "проблемы с сотрудниками" --depth 3
  media2rag search "query" --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func runSearch(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	query := args[0]

	// Setup clients
	llmClient, embedClient, qClient, kg, communities, err := setupSearch(ctx, query)
	if err != nil {
		return err
	}

	// 1. RAG search (uses embed client for query vector)
	chunks, ragErr := searchChunks(ctx, query, qClient, embedClient)

	// 2. Try GraphRAG path
	var graphResult *model.GraphRAGResult
	var graphErr error

	if kg != nil {
		graphResult, graphErr = searchGraph(ctx, query, chunks, kg, communities, llmClient)
	}

	// 3. Handle fallbacks
	if ragErr != nil && graphErr != nil {
		return fmt.Errorf("RAG: %w, GraphRAG: %v", ragErr, graphErr)
	}

	if ragErr != nil {
		// RAG failed, show GraphRAG only
		return outputGraphRAGText(graphResult)
	}

	if graphErr != nil || graphResult == nil || len(graphResult.Chains) == 0 {
		// Graph failed or empty, show RAG only
		return outputText(chunks)
	}

	// 4. Hybrid: synthesize chunks + chains
	return outputHybrid(ctx, query, chunks, graphResult, llmClient)
}

func setupSearch(ctx context.Context, query string) (llmClient llm.LLMClient, embedClient llm.LLMClient, qClient *qdrant.Client, kg *model.KnowledgeGraph, communities []*model.Community, err error) {
	backend := searchBackend
	if backend == "" {
		backend = cfg.LLM.DefaultBackend
	}
	modelName := searchModel
	if modelName == "" {
		modelName = cfg.LLM.Model
	}

	llmClient, err = llm.NewClient(
		ctx, backend, cfg.LLM.OllamaURL, modelName,
		cfg.LLM.OpenRouterURL, cfg.LLM.OpenRouterKey,
		cfg.LLM.LMStudioURL, time.Duration(cfg.LLM.Timeout)*time.Second,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("init LLM client: %w", err)
	}

	// Separate embed client (for RAG query embedding)
	embedBackend := searchEmbedBackend
	if embedBackend == "" {
		embedBackend = backend
	}
	embedModel := searchEmbedModel
	if embedModel == "" {
		embedModel = modelName
	}
	embedClient = llmClient // default: reuse LLM client
	if searchEmbedBackend != "" || searchEmbedModel != "" {
		embedClient, err = llm.NewClient(
			ctx, embedBackend, cfg.LLM.OllamaURL, embedModel,
			cfg.LLM.OpenRouterURL, cfg.LLM.OpenRouterKey,
			cfg.LLM.LMStudioURL, time.Duration(cfg.LLM.Timeout)*time.Second,
		)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("init embed client: %w", err)
		}
	}

	// Qdrant client
	qURL := searchQdrantURL
	if qURL == "" {
		qURL = "http://localhost:6333"
	}
	qClient = qdrant.NewClient(qURL, searchQdrantAPIKey, searchCollection)
	if !qClient.IsAvailable(ctx) {
		qClient = nil // Will fallback to GraphRAG only
	}

	// Graph
	gPath := searchGraphPath
	if gPath == "" {
		home, _ := os.UserHomeDir()
		gPath = filepath.Join(home, ".media2rag", "data", "graph.json")
	}

	if graph.GraphExists(gPath) {
		kg, err = graph.LoadGraph(gPath)
		if err != nil {
			kg = nil
		}
	}

	cPath := searchCommunitiesPath
	if cPath == "" {
		home, _ := os.UserHomeDir()
		cPath = filepath.Join(home, ".media2rag", "data", "graph_communities.json")
	}

	if graph.GraphExists(cPath) {
		communities, err = graph.LoadCommunities(cPath)
		if err != nil {
			communities = nil
		}
	}

	return llmClient, embedClient, qClient, kg, communities, nil
}

func searchChunks(ctx context.Context, query string, qClient *qdrant.Client, embedClient llm.LLMClient) ([]rrfResult, error) {
	if qClient == nil {
		return nil, fmt.Errorf("Qdrant not available")
	}

	queryEmbed, err := embedClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	limit := searchTop * 2

	denseResults, err := qClient.Search(ctx, queryEmbed, limit, nil)
	if err != nil {
		return nil, fmt.Errorf("dense search: %w", err)
	}

	sparseResults, err := qClient.SearchSparse(ctx, query, limit)
	if err != nil {
		sparseResults = nil
	}

	return rrfFusion(denseResults, sparseResults, searchTop, searchMinScore), nil
}

func extractEntitiesFromChunks(ctx context.Context, chunks []rrfResult, client llm.LLMClient, modelName string) ([]string, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	var sb strings.Builder
	for i, c := range chunks {
		content := extractPayloadString(c.Point.Payload, "content")
		topic := extractPayloadString(c.Point.Payload, "topic")
		sb.WriteString(fmt.Sprintf("Chunk %d (topic: %s):\n%s\n\n", i+1, topic, truncate(content, 300)))
	}

	prompt := fmt.Sprintf(`Extract entity names from these text chunks that could match nodes in a knowledge graph.

Knowledge graph node types: Problem, Solution, Opportunity, Skill, Resource, Market, Audience, Business, Event, Claim, Metric, Concept

Return ONLY comma-separated entity names, nothing else.

Chunks:
%s

Entities:`, sb.String())

	resp, err := client.Chat(ctx, model.ChatRequest{
		Model: modelName,
		Messages: []model.Message{
			{Role: "system", Content: "Extract entity names from text. Return comma-separated list only."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, err
	}

	var entities []string
	for _, e := range strings.Split(resp.Message.Content, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			entities = append(entities, e)
		}
	}

	return entities, nil
}

func searchGraph(ctx context.Context, query string, chunks []rrfResult, kg *model.KnowledgeGraph, communities []*model.Community, client llm.LLMClient) (*model.GraphRAGResult, error) {
	// Extract entities from chunks
	entities, err := extractEntitiesFromChunks(ctx, chunks, client, cfg.LLM.Model)
	if err != nil {
		// Fallback: use query keywords
		rewriter := graph.NewQueryRewriter(client, cfg.LLM.Model)
		gq, rewriteErr := rewriter.Rewrite(ctx, query)
		if rewriteErr != nil {
			return nil, fmt.Errorf("extract entities: %v, rewrite: %v", err, rewriteErr)
		}
		entities = gq.Entities
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no entities extracted")
	}

	// Find seed entities
	seeds := findSeedEntities(entities, kg)
	if len(seeds) == 0 {
		return nil, fmt.Errorf("no matching entities in graph")
	}

	// Use QueryRewriter for pattern/mode
	rewriter := graph.NewQueryRewriter(client, cfg.LLM.Model)
	gq, err := rewriter.Rewrite(ctx, query)
	if err != nil {
		gq = &model.GraphQuery{
			Entities: entities,
			Pattern:  model.PatternRootCause,
			Mode:     model.ModeLocal,
			Depth:    2,
		}
	}

	if searchDepth > 0 {
		gq.Depth = searchDepth
	}

	// Execute based on mode
	switch gq.Mode {
	case model.ModeGlobal:
		return globalSearch(ctx, query, gq, kg, communities, client, cfg.LLM.Model), nil
	case model.ModeDRIFT:
		return driftSearch(ctx, query, gq, kg, communities, client, cfg.LLM.Model), nil
	default:
		return localSearch(ctx, query, gq, kg, client, cfg.LLM.Model), nil
	}
}

func outputHybrid(ctx context.Context, query string, chunks []rrfResult, graphResult *model.GraphRAGResult, client llm.LLMClient) error {
	if searchFormat == "json" {
		return outputHybridJSON(query, chunks, graphResult)
	}

	// LLM synthesis
	answer, err := synthesizeHybrid(ctx, query, chunks, graphResult, client, cfg.LLM.Model)
	if err != nil {
		// Fallback to plain text
		return outputHybridPlain(query, chunks, graphResult)
	}

	fmt.Printf("Hybrid Search: %s\n\n", query)

	if len(graphResult.Chains) > 0 {
		fmt.Printf("Graph: %d reasoning chains\n", len(graphResult.Chains))
		for i, chain := range graphResult.Chains {
			fmt.Printf("  %d. %s (conf: %.2f)\n", i+1, strings.Join(chain.Path, " → "), chain.Confidence)
		}
		fmt.Println()
	}

	if len(chunks) > 0 {
		fmt.Printf("RAG: %d relevant chunks\n", len(chunks))
		for i, c := range chunks {
			topic := extractPayloadString(c.Point.Payload, "topic")
			content := extractPayloadString(c.Point.Payload, "content")
			fmt.Printf("  %d. [%s] %s\n", i+1, topic, truncate(content, 120))
		}
		fmt.Println()
	}

	fmt.Println("---\n")
	fmt.Println(answer)

	if len(graphResult.Provenance) > 0 {
		fmt.Printf("\nProvenance: %d source chunks\n", len(graphResult.Provenance))
	}

	return nil
}

func synthesizeHybrid(ctx context.Context, query string, chunks []rrfResult, graphResult *model.GraphRAGResult, client llm.LLMClient, modelName string) (string, error) {
	var chunksText strings.Builder
	for i, c := range chunks {
		content := extractPayloadString(c.Point.Payload, "content")
		topic := extractPayloadString(c.Point.Payload, "topic")
		chunksText.WriteString(fmt.Sprintf("Chunk %d (%s):\n%s\n\n", i+1, topic, truncate(content, 400)))
	}

	var chainsText strings.Builder
	for i, chain := range graphResult.Chains {
		chainsText.WriteString(fmt.Sprintf("Chain %d: %s\n", i+1, strings.Join(chain.Path, " → ")))
		if len(chain.Mechanisms) > 0 {
			chainsText.WriteString(fmt.Sprintf("  Mechanisms: %s\n", strings.Join(chain.Mechanisms, "; ")))
		}
		chainsText.WriteString(fmt.Sprintf("  Confidence: %.2f\n\n", chain.Confidence))
	}

	prompt := fmt.Sprintf(`Answer the user's query using both semantic search results and knowledge graph reasoning chains.

Query: %s

RAG Context (relevant document chunks):
%s

Graph Reasoning Chains (causal relationships):
%s

Synthesize a comprehensive answer that:
1. Uses the RAG chunks for detailed context and examples
2. Uses the graph chains to show causal relationships and connections
3. Combines both into a clear, actionable response

Answer:`, query, chunksText.String(), chainsText.String())

	resp, err := client.Chat(ctx, model.ChatRequest{
		Model: modelName,
		Messages: []model.Message{
			{Role: "system", Content: "You are an AI assistant that synthesizes semantic search results with knowledge graph reasoning chains."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Message.Content), nil
}

func outputHybridJSON(query string, chunks []rrfResult, graphResult *model.GraphRAGResult) error {
	output := map[string]interface{}{
		"query":  query,
		"mode":   "hybrid",
		"rag":    chunks,
		"graph":  graphResult,
		"total_chunks": len(chunks),
		"total_chains": len(graphResult.Chains),
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func outputHybridPlain(query string, chunks []rrfResult, graphResult *model.GraphRAGResult) error {
	fmt.Printf("Hybrid Search: %s\n\n", query)

	if len(graphResult.Chains) > 0 {
		fmt.Printf("Graph: %d reasoning chains\n", len(graphResult.Chains))
		for i, chain := range graphResult.Chains {
			fmt.Printf("  %d. %s (conf: %.2f)\n", i+1, strings.Join(chain.Path, " → "), chain.Confidence)
		}
		fmt.Println()
	}

	if len(chunks) > 0 {
		fmt.Printf("RAG: %d relevant chunks\n", len(chunks))
		for i, c := range chunks {
			topic := extractPayloadString(c.Point.Payload, "topic")
			content := extractPayloadString(c.Point.Payload, "content")
			fmt.Printf("  %d. [%s] %s\n", i+1, topic, truncate(content, 120))
		}
		fmt.Println()
	}

	return nil
}

func init() {
	home, _ := os.UserHomeDir()

	searchCmd.Flags().StringVar(&searchQdrantURL, "qdrant-url", "", "Qdrant URL (default: http://localhost:6333)")
	searchCmd.Flags().StringVar(&searchQdrantAPIKey, "qdrant-api-key", "", "Qdrant API key")
	searchCmd.Flags().StringVar(&searchCollection, "collection", "media2rag", "Qdrant collection name")
	searchCmd.Flags().IntVar(&searchTop, "top", 5, "Number of RAG results")
	searchCmd.Flags().Float64Var(&searchMinScore, "min-score", 0.0, "Minimum score threshold")
	searchCmd.Flags().StringVar(&searchGraphPath, "graph-path", filepath.Join(home, ".media2rag", "data", "graph.json"), "Path to graph.json")
	searchCmd.Flags().StringVar(&searchCommunitiesPath, "communities-path", filepath.Join(home, ".media2rag", "data", "graph_communities.json"), "Path to communities.json")
	searchCmd.Flags().IntVar(&searchDepth, "depth", 0, "Traversal depth (default: auto)")
	searchCmd.Flags().StringVar(&searchFormat, "format", "text", "Output format: text, json")
	searchCmd.Flags().StringVar(&searchBackend, "backend", "", "LLM backend for chat & synthesis")
	searchCmd.Flags().StringVar(&searchModel, "model", "", "LLM model for chat & synthesis")
	searchCmd.Flags().StringVar(&searchEmbedBackend, "embed-backend", "", "Embedding backend for query (default: same as --backend)")
	searchCmd.Flags().StringVar(&searchEmbedModel, "embed-model", "", "Embedding model for query (default: same as --model)")

	rootCmd.AddCommand(searchCmd)
}
