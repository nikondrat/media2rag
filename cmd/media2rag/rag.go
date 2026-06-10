package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/llm"
	"media2rag/internal/qdrant"
)

var (
	ragQdrantURL    string
	ragQdrantAPIKey string
	ragCollection   string
	ragTop          int
	ragMinScore     float64
	ragFormat       string
	ragBackend      string
	ragModel        string
)

var ragCmd = &cobra.Command{
	Use:   "rag <query>",
	Short: "Search indexed chunks using hybrid vector search",
	Long: `Search through indexed document chunks using hybrid search (dense + sparse + RRF fusion).

Usage:
  media2rag rag "how to scale a business"
  media2rag rag "sales metrics" --top 10 --min-score 0.7
  media2rag rag "query" --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runRAG,
}

func runRAG(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	query := args[0]

	// Setup Qdrant client
	qdrantURL := ragQdrantURL
	if qdrantURL == "" {
		qdrantURL = "http://localhost:6333"
	}

	qClient := qdrant.NewClient(qdrantURL, ragQdrantAPIKey, ragCollection)

	if !qClient.IsAvailable(ctx) {
		return fmt.Errorf("Qdrant not available at %s, run: media2rag index", qdrantURL)
	}

	// Setup embed client
	backend := ragBackend
	if backend == "" {
		backend = cfg.LLM.DefaultBackend
	}
	modelName := ragModel
	if modelName == "" {
		modelName = cfg.LLM.Model
	}

	embedClient, err := llm.NewClient(
		ctx,
		backend,
		cfg.LLM.OllamaURL,
		modelName,
		cfg.LLM.OpenRouterURL,
		cfg.LLM.OpenRouterKey,
		cfg.LLM.LMStudioURL,
		time.Duration(cfg.LLM.Timeout)*time.Second,
	)
	if err != nil {
		return fmt.Errorf("init embed client: %w", err)
	}

	// Embed query
	queryEmbed, err := embedClient.Embed(ctx, query)
	if err != nil {
		return fmt.Errorf("embed query: %w", err)
	}

	// Hybrid search: dense + sparse + RRF fusion
	limit := ragTop * 2 // Get more results for RRF fusion

	// Dense search
	denseResults, err := qClient.Search(ctx, queryEmbed, limit, nil)
	if err != nil {
		return fmt.Errorf("dense search: %w", err)
	}

	// Sparse search (keyword-based for now)
	sparseResults, err := qClient.SearchSparse(ctx, query, limit)
	if err != nil {
		sparseResults = nil // Non-fatal
	}

	// RRF fusion
	results := rrfFusion(denseResults, sparseResults, ragTop, ragMinScore)

	// Output
	switch ragFormat {
	case "json":
		return outputJSON(results)
	default:
		return outputText(results)
	}
}

// rrfResult combines dense and sparse scores using RRF
type rrfResult struct {
	Point   qdrant.SearchResult
	DenseRank   int
	SparseRank  int
	RRFScore    float64
}

// rrfFusion fuses dense and sparse results using Reciprocal Rank Fusion
func rrfFusion(denseResults, sparseResults []qdrant.SearchResult, topK int, minScore float64) []rrfResult {
	const k = 60 // RRF constant

	// Build sparse rank map
	sparseRank := make(map[uint64]int)
	for i, r := range sparseResults {
		sparseRank[r.ID] = i + 1
	}

	// Compute RRF scores
	var results []rrfResult
	seen := make(map[uint64]bool)

	for i, r := range denseResults {
		seen[r.ID] = true
		denseRank := i + 1
		sr, hasSparse := sparseRank[r.ID]
		if !hasSparse {
			sr = len(sparseResults) + 1
		}

		rrfScore := 1.0/(float64(k)+float64(denseRank)) + 1.0/(float64(k)+float64(sr))
		results = append(results, rrfResult{
			Point:      r,
			DenseRank:  denseRank,
			SparseRank: sr,
			RRFScore:   rrfScore,
		})
	}

	// Add sparse-only results
	for i, r := range sparseResults {
		if seen[r.ID] {
			continue
		}
		rrfScore := 1.0/(float64(k)+float64(len(denseResults)+1)) + 1.0/(float64(k)+float64(i+1))
		results = append(results, rrfResult{
			Point:      r,
			DenseRank:  len(denseResults) + 1,
			SparseRank: i + 1,
			RRFScore:   rrfScore,
		})
	}

	// Sort by RRF score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].RRFScore > results[j].RRFScore
	})

	// Apply min score filter and limit
	var filtered []rrfResult
	for _, r := range results {
		if r.Point.Score < minScore {
			continue
		}
		filtered = append(filtered, r)
		if len(filtered) >= topK {
			break
		}
	}

	return filtered
}

// ragOutput represents the JSON output format
type ragOutput struct {
	Query   string              `json:"query"`
	Results []ragResultItem     `json:"results"`
	Total   int                 `json:"total"`
}

type ragResultItem struct {
	ChunkID   string   `json:"chunk_id"`
	File      string   `json:"file"`
	Score     float64  `json:"score"`
	RRFScore  float64  `json:"rrf_score"`
	Topic     string   `json:"topic"`
	Summary   string   `json:"summary"`
	KeyPoints []string `json:"key_points,omitempty"`
	Content   string   `json:"content"`
}

func outputJSON(results []rrfResult) error {
	output := ragOutput{
		Results: make([]ragResultItem, len(results)),
		Total:   len(results),
	}

	for i, r := range results {
		item := ragResultItem{
			ChunkID:  fmt.Sprintf("%d", r.Point.ID),
			Score:    r.Point.Score,
			RRFScore: r.RRFScore,
			Content:  extractPayloadString(r.Point.Payload, "content"),
			Topic:    extractPayloadString(r.Point.Payload, "topic"),
			File:     extractPayloadString(r.Point.Payload, "doc_id"),
		}
		output.Results[i] = item
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func outputText(results []rrfResult) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d results:\n\n", len(results))

	for i, r := range results {
		content := extractPayloadString(r.Point.Payload, "content")
		topic := extractPayloadString(r.Point.Payload, "topic")
		docID := extractPayloadString(r.Point.Payload, "doc_id")

		fmt.Printf("--- Result %d (score: %.4f, rrf: %.4f) ---\n", i+1, r.Point.Score, r.RRFScore)
		fmt.Printf("Topic: %s | Document: %s\n", topic, docID)
		fmt.Printf("Content:\n%s\n\n", truncate(content, 500))
	}

	return nil
}

func extractPayloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func init() {
	ragCmd.Flags().StringVar(&ragQdrantURL, "qdrant-url", "", "Qdrant URL (default: http://localhost:6333)")
	ragCmd.Flags().StringVar(&ragQdrantAPIKey, "qdrant-api-key", "", "Qdrant API key")
	ragCmd.Flags().StringVar(&ragCollection, "collection", "media2rag", "Qdrant collection name")
	ragCmd.Flags().IntVar(&ragTop, "top", 5, "Number of results to return")
	ragCmd.Flags().Float64Var(&ragMinScore, "min-score", 0.0, "Minimum score threshold")
	ragCmd.Flags().StringVar(&ragFormat, "format", "text", "Output format (text, json)")
	ragCmd.Flags().StringVar(&ragBackend, "backend", "", "LLM backend for embedding")
	ragCmd.Flags().StringVar(&ragModel, "model", "", "LLM model for embedding")
	rootCmd.AddCommand(ragCmd)
}
