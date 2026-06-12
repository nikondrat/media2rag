package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/graph"
	"media2rag/internal/llm"
)

var (
	updateGraphPath       string
	updateCommunitiesPath string
	updateBackend         string
	updateModel           string
	updateEmbedBackend    string
	updateEmbedModel      string
)

var updateCmd = &cobra.Command{
	Use:   "update <file.md> [file2.md ...]",
	Short: "Incrementally update graph with new documents",
	Long: `Process new documents and merge them into the existing knowledge graph.
Unlike 'index --incremental', this command is designed for daily use with individual files.

Usage:
  media2rag update ./new-doc.md
  media2rag update ./doc1.md ./doc2.md
  media2rag update ./docs/*.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Setup paths
	if updateGraphPath == "" {
		home, _ := os.UserHomeDir()
		updateGraphPath = filepath.Join(home, ".media2rag", "data", "graph.json")
	}
	if updateCommunitiesPath == "" {
		home, _ := os.UserHomeDir()
		updateCommunitiesPath = filepath.Join(home, ".media2rag", "data", "graph_communities.json")
	}

	// Setup LLM clients
	backend := updateBackend
	if backend == "" {
		backend = cfg.LLM.DefaultBackend
	}
	modelName := updateModel
	if modelName == "" {
		modelName = cfg.LLM.Model
	}
	embedModelName := updateEmbedModel
	if embedModelName == "" {
		embedModelName = modelName
	}
	embedBackend := updateEmbedBackend
	if embedBackend == "" {
		embedBackend = backend
	}

	extractClient, err := llm.NewClient(
		ctx, backend, cfg.LLM.OllamaURL, modelName,
		cfg.LLM.OpenRouterURL, cfg.LLM.OpenRouterKey, cfg.LLM.LMStudioURL,
		time.Duration(cfg.LLM.Timeout)*time.Second,
	)
	if err != nil {
		return fmt.Errorf("init LLM client: %w", err)
	}

	embedClient, err := llm.NewClient(
		ctx, embedBackend, cfg.LLM.OllamaURL, embedModelName,
		cfg.LLM.OpenRouterURL, cfg.LLM.OpenRouterKey, cfg.LLM.LMStudioURL,
		time.Duration(cfg.LLM.Timeout)*time.Second,
	)
	if err != nil {
		return fmt.Errorf("init embed client: %w", err)
	}

	// Read and parse input files
	var chunks []graph.ChunkInput
	var chunksWithTopic []graph.ChunkWithTopic

	for _, filePath := range args {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}

		docID := filepath.Base(filePath)
		parsed := parseChunksFromMarkdown(string(content), docID)

		for i, ch := range parsed {
			chunkID := fmt.Sprintf("%s:chunk_%d", docID, i)
			chunkText := ch.ChunkText()
			chunks = append(chunks, graph.ChunkInput{
				DocID:      docID,
				ChunkIndex: i,
				Content:    chunkText,
				Topic:      ch.Topic,
			})
			chunksWithTopic = append(chunksWithTopic, graph.ChunkWithTopic{
				ID:        chunkID,
				Topic:     ch.Topic,
				Summary:   ch.Summary,
				KeyPoints: ch.KeyPoints,
				Content:   chunkText,
			})
		}
	}

	if len(chunks) == 0 {
		return fmt.Errorf("no chunks found in provided files")
	}

	fmt.Fprintf(os.Stderr, "Processing %d chunks from %d files...\n", len(chunks), len(args))

	// Build components
	store := graph.NewJSONStore()
	extractor := graph.NewEntityExtractor(extractClient, 5, modelName)
	resolver := graph.NewEntityResolver(extractClient, embedClient, modelName)
	detector := graph.NewCommunityDetector(extractClient, modelName)

	// Run incremental update
	updater := graph.NewIncrementalUpdater(store, extractor, resolver, detector)
	result, err := updater.Update(ctx, updateGraphPath, updateCommunitiesPath, chunks, chunksWithTopic)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n✓ Update complete!\n")
	fmt.Fprintf(os.Stderr, "  Nodes: +%d merged, +%d new (total: %d)\n", result.NodesMerged, result.NodesAdded, result.TotalNodes)
	fmt.Fprintf(os.Stderr, "  Edges: +%d (total: %d)\n", result.EdgesAdded, result.TotalEdges)
	fmt.Fprintf(os.Stderr, "  Communities: +%d updated\n", result.CommunitiesUp)

	return nil
}

func init() {
	home, _ := os.UserHomeDir()
	defaultGraphPath := filepath.Join(home, ".media2rag", "data", "graph.json")

	updateCmd.Flags().StringVar(&updateGraphPath, "graph-path", defaultGraphPath, "Path to graph.json")
	updateCmd.Flags().StringVar(&updateCommunitiesPath, "communities-path", "", "Path to communities.json")
	updateCmd.Flags().StringVar(&updateBackend, "backend", "", "LLM backend")
	updateCmd.Flags().StringVar(&updateModel, "model", "", "LLM model")
	updateCmd.Flags().StringVar(&updateEmbedBackend, "embed-backend", "", "Embedding backend")
	updateCmd.Flags().StringVar(&updateEmbedModel, "embed-model", "", "Embedding model")
	rootCmd.AddCommand(updateCmd)
}
