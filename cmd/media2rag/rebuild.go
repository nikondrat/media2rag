package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/graph"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/qdrant"
)

var (
	rebuildQdrantURL       string
	rebuildQdrantAPIKey    string
	rebuildCollection      string
	rebuildGraphPath       string
	rebuildCommunitiesPath string
	rebuildConcurrency     int
	rebuildBackend         string
	rebuildModel           string
	rebuildEmbedModel      string
	rebuildEmbedBackend    string
	rebuildClusterMethod   string
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild [dir]",
	Short: "Full rebuild of knowledge graph (ignores incremental cache)",
	Long: `Completely rebuild the knowledge graph from scratch.
Unlike 'index --incremental', this ignores all cached data and rebuilds everything.

Use when:
- Graph is corrupted
- You want to switch clustering methods
- You want to re-extract with a different LLM model

Usage:
  media2rag rebuild                              # rebuild default workspace
  media2rag rebuild /path/to/results/            # rebuild flat .md files
  media2rag rebuild --cluster topic              # use topic clustering instead of leiden`,
	Args: cobra.MaximumNArgs(1),
	RunE:  runRebuild,
}

func runRebuild(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Determine input directory
	inputDir := cfg.Workspace.DataDir
	if inputDir == "" {
		inputDir = filepath.Join(os.Getenv("HOME"), ".media2rag", "workspace")
	}
	if len(args) > 0 {
		inputDir = args[0]
	}

	// Determine output paths
	dataDir := filepath.Join(inputDir, "data")
	if rebuildGraphPath == "" {
		rebuildGraphPath = filepath.Join(dataDir, "graph.json")
	}
	if rebuildCommunitiesPath == "" {
		rebuildCommunitiesPath = filepath.Join(dataDir, "graph_communities.json")
	}

	// Clear existing cache
	manifestPath := filepath.Join(filepath.Dir(rebuildGraphPath), "chunk_manifest.json")
	_ = os.Remove(manifestPath)
	fmt.Fprintf(os.Stderr, "Cleared incremental cache\n")

	// Setup LLM clients
	backend := rebuildBackend
	if backend == "" {
		backend = cfg.LLM.DefaultBackend
	}
	modelName := rebuildModel
	if modelName == "" {
		modelName = cfg.LLM.Model
	}
	embedModelName := rebuildEmbedModel
	if embedModelName == "" {
		embedModelName = modelName
	}
	embedBackend := rebuildEmbedBackend
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

	// Collect chunks
	var allChunks []graph.ChunkInput
	var allChunksWithTopic []graph.ChunkWithTopic

	isWorkspace := isWorkspaceDir(inputDir)

	if isWorkspace {
		fmt.Fprintf(os.Stderr, "Detected workspace directory: %s\n", inputDir)
		allChunks, allChunksWithTopic, err = collectWorkspaceChunks(inputDir)
	} else {
		fmt.Fprintf(os.Stderr, "Detected flat .md files directory: %s\n", inputDir)
		allChunks, allChunksWithTopic, err = collectFlatFileChunks(inputDir)
	}
	if err != nil {
		return err
	}

	if len(allChunks) == 0 {
		return fmt.Errorf("no chunks found in %s", inputDir)
	}

	fmt.Fprintf(os.Stderr, "Collected %d chunks\n", len(allChunks))

	// --- Qdrant indexing ---
	qdrantURL := rebuildQdrantURL
	if qdrantURL == "" {
		qdrantURL = "http://localhost:6333"
	}

	qClient := qdrant.NewClient(qdrantURL, rebuildQdrantAPIKey, rebuildCollection)

	if !qClient.IsAvailable(ctx) {
		fmt.Fprintf(os.Stderr, "Warning: Qdrant not available at %s, skipping vector indexing\n", qdrantURL)
	} else {
		fmt.Fprintf(os.Stderr, "Qdrant available, initializing collection...\n")

		vectorSize := detectVectorSize(modelName)
		if err := qClient.InitCollection(ctx, vectorSize); err != nil {
			return fmt.Errorf("init Qdrant collection: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Embedding and indexing %d chunks...\n", len(allChunks))

		points := make([]qdrant.Point, 0, len(allChunks))
		for _, chunk := range allChunks {
			embedding, err := embedClient.Embed(ctx, chunk.Content)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: embed failed for %s: %v\n", chunk.DocID, err)
				continue
			}

			chunkID := fmt.Sprintf("%s:chunk_%d", chunk.DocID, chunk.ChunkIndex)
			pointID := stringToUint64(chunkID)
			points = append(points, qdrant.Point{
				ID:     pointID,
				Vector: embedding,
				Payload: map[string]interface{}{
					"content":   chunk.Content,
					"doc_id":    chunk.DocID,
					"topic":     chunk.Topic,
					"chunk_idx": chunk.ChunkIndex,
				},
			})
		}

		if len(points) > 0 {
			if err := qClient.Upsert(ctx, points); err != nil {
				return fmt.Errorf("upsert to Qdrant: %w", err)
			}
			count, _ := qClient.PointCount(ctx)
			fmt.Fprintf(os.Stderr, "Indexed %d chunks into Qdrant (total: %d)\n", len(points), count)
		}
	}

	// --- Graph extraction ---
	fmt.Fprintf(os.Stderr, "Extracting entities and relations from %d chunks (%d concurrent)...\n", len(allChunks), rebuildConcurrency)

	checkpointDir := filepath.Join(filepath.Dir(rebuildGraphPath), "extractions")
	extractor := graph.NewEntityExtractorWithCheckpoint(extractClient, rebuildConcurrency, modelName, checkpointDir)
	results := extractor.ExtractBatchWithCheckpoint(ctx, allChunks, nil, nil)

	// Build graph
	graphData := model.NewKnowledgeGraph()
	for _, result := range results {
		if result.Err != nil {
			continue
		}
		for _, node := range result.Nodes {
			graphData.AddNode(node)
		}
		for _, edge := range result.Edges {
			graphData.AddEdge(edge)
		}
	}

	fmt.Fprintf(os.Stderr, "Extracted %d nodes, %d edges\n", len(graphData.Nodes), len(graphData.Edges))

	// Deduplicate
	fmt.Fprintf(os.Stderr, "Deduplicating entities...\n")
	resolver := graph.NewEntityResolver(extractClient, embedClient, modelName)
	dedupedNodes, err := resolver.Resolve(ctx, graphData.Nodes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: entity resolution failed: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Deduplicated: %d → %d nodes\n", len(graphData.Nodes), len(dedupedNodes))
		graphData = model.NewKnowledgeGraph()
		for _, node := range dedupedNodes {
			graphData.AddNode(node)
		}
		for _, result := range results {
			if result.Err != nil {
				continue
			}
			for _, edge := range result.Edges {
				if _, fromOk := graphData.GetNodeByID(edge.From); fromOk {
					if _, toOk := graphData.GetNodeByID(edge.To); toOk {
						graphData.AddEdge(edge)
					}
				}
			}
		}
	}

	graphData.BuildIndexes()
	if err := graphData.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: graph validation: %v\n", err)
	}

	// Save graph
	if err := os.MkdirAll(filepath.Dir(rebuildGraphPath), 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := graph.SaveGraph(graphData, rebuildGraphPath); err != nil {
		return fmt.Errorf("save graph: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Saved graph to %s\n", rebuildGraphPath)

	// --- Community detection ---
	fmt.Fprintf(os.Stderr, "Detecting communities (method: %s)...\n", rebuildClusterMethod)

	detector := graph.NewCommunityDetector(extractClient, modelName)
	var communities []*model.Community

	if rebuildClusterMethod == "leiden" {
		leiden := graph.NewLeidenCluster(1.0, 10)
		communities = leiden.Detect(graphData)
	} else {
		communities = detector.DetectGroups(allChunksWithTopic)
	}
	fmt.Fprintf(os.Stderr, "Total communities: %d\n", len(communities))

	// Generate summaries
	fmt.Fprintf(os.Stderr, "Generating community summaries...\n")
	if err := detector.GenerateSummaries(ctx, communities, allChunksWithTopic); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: community summary generation: %v\n", err)
	}

	if err := detector.GenerateDomainHierarchy(ctx, communities); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: domain hierarchy generation: %v\n", err)
	}

	graph.LinkCommunitiesToGraph(communities, graphData)

	if err := os.MkdirAll(filepath.Dir(rebuildCommunitiesPath), 0755); err != nil {
		return fmt.Errorf("create communities dir: %w", err)
	}
	if err := graph.SaveCommunities(communities, rebuildCommunitiesPath); err != nil {
		return fmt.Errorf("save communities: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Saved communities to %s\n", rebuildCommunitiesPath)

	fmt.Fprintf(os.Stderr, "\n✓ Rebuild complete!\n")
	fmt.Fprintf(os.Stderr, "  Graph: %s\n", rebuildGraphPath)
	fmt.Fprintf(os.Stderr, "  Communities: %s\n", rebuildCommunitiesPath)
	fmt.Fprintf(os.Stderr, "  Nodes: %d, Edges: %d, Communities: %d\n", len(graphData.Nodes), len(graphData.Edges), len(communities))

	return nil
}

func init() {
	rebuildCmd.Flags().StringVar(&rebuildQdrantURL, "qdrant-url", "", "Qdrant URL (default: http://localhost:6333)")
	rebuildCmd.Flags().StringVar(&rebuildQdrantAPIKey, "qdrant-api-key", "", "Qdrant API key")
	rebuildCmd.Flags().StringVar(&rebuildCollection, "collection", "media2rag", "Qdrant collection name")
	rebuildCmd.Flags().StringVar(&rebuildGraphPath, "graph-path", "", "Path to save graph.json")
	rebuildCmd.Flags().StringVar(&rebuildCommunitiesPath, "communities-path", "", "Path to save communities.json")
	rebuildCmd.Flags().IntVar(&rebuildConcurrency, "concurrency", 5, "Concurrent LLM requests")
	rebuildCmd.Flags().StringVar(&rebuildBackend, "backend", "", "LLM backend")
	rebuildCmd.Flags().StringVar(&rebuildModel, "model", "", "LLM model for extraction")
	rebuildCmd.Flags().StringVar(&rebuildEmbedModel, "embed-model", "", "Embedding model (defaults to --model)")
	rebuildCmd.Flags().StringVar(&rebuildEmbedBackend, "embed-backend", "", "Embedding backend (defaults to --backend)")
	rebuildCmd.Flags().StringVar(&rebuildClusterMethod, "cluster", "leiden", "Clustering method: leiden (default), topic")
	rootCmd.AddCommand(rebuildCmd)
}
