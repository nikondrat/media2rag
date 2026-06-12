package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"media2rag/internal/graph"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/qdrant"
	"media2rag/internal/workspace"
)

var (
	indexQdrantURL       string
	indexQdrantAPIKey    string
	indexCollection      string
	indexGraphPath       string
	indexCommunitiesPath string
	indexIncremental     bool
	indexConcurrency     int
	indexBackend         string
	indexModel           string
	indexEmbedModel      string
	indexEmbedBackend    string
)

var indexCmd = &cobra.Command{
	Use:   "index [dir]",
	Short: "Index all RAGDocuments into Qdrant and build Knowledge Graph",
	Long: `Index all processed documents into Qdrant for vector search
and build a Knowledge Graph with entities and relations extracted from chunks.

Accepts either a workspace directory (created by 'media2rag process') or
a directory of flat .md files with chunk frontmatter.

Usage:
  media2rag index                              # index from default workspace
  media2rag index /path/to/results/            # index flat .md files
  media2rag index ~/.media2rag/workspace       # index workspace
  media2rag index --incremental                # only index new/changed documents`,
	Args: cobra.MaximumNArgs(1),
	RunE:  runIndex,
}

func runIndex(cmd *cobra.Command, args []string) error {
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
	if indexGraphPath == "" {
		indexGraphPath = filepath.Join(dataDir, "graph.json")
	}
	if indexCommunitiesPath == "" {
		indexCommunitiesPath = filepath.Join(dataDir, "graph_communities.json")
	}

	// Setup LLM clients
	backend := indexBackend
	if backend == "" {
		backend = cfg.LLM.DefaultBackend
	}
	modelName := indexModel
	if modelName == "" {
		modelName = cfg.LLM.Model
	}
	embedModelName := indexEmbedModel
	if embedModelName == "" {
		embedModelName = modelName
	}
	embedBackend := indexEmbedBackend
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

	// Collect chunks from input
	var allChunks []graph.ChunkInput
	var allChunksWithTopic []graph.ChunkWithTopic

	// Detect input type: workspace or flat .md files
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
		fmt.Fprintf(os.Stderr, "No chunks found in %s\n", inputDir)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Collected %d chunks\n", len(allChunks))

	// --- Qdrant indexing ---
	qdrantURL := indexQdrantURL
	if qdrantURL == "" {
		qdrantURL = "http://localhost:6333"
	}

	qClient := qdrant.NewClient(qdrantURL, indexQdrantAPIKey, indexCollection)

	if !qClient.IsAvailable(ctx) {
		fmt.Fprintf(os.Stderr, "Warning: Qdrant not available at %s\n", qdrantURL)
		fmt.Fprintf(os.Stderr, "Skipping vector indexing. Run Qdrant and retry for vector search.\n")
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
	fmt.Fprintf(os.Stderr, "Extracting entities and relations from %d chunks...\n", len(allChunks))

	extractor := graph.NewEntityExtractor(extractClient, indexConcurrency, modelName)
	results := extractor.ExtractBatch(ctx, allChunks, nil)

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
	if err := os.MkdirAll(filepath.Dir(indexGraphPath), 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := graph.SaveGraph(graphData, indexGraphPath); err != nil {
		return fmt.Errorf("save graph: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Saved graph to %s\n", indexGraphPath)

	// --- Community detection ---
	fmt.Fprintf(os.Stderr, "Detecting communities...\n")

	detector := graph.NewCommunityDetector(extractClient, modelName)
	communities := detector.DetectGroups(allChunksWithTopic)
	fmt.Fprintf(os.Stderr, "Found %d communities\n", len(communities))

	fmt.Fprintf(os.Stderr, "Generating community summaries...\n")
	if err := detector.GenerateSummaries(ctx, communities, allChunksWithTopic); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: community summary generation: %v\n", err)
	}

	if err := detector.GenerateDomainHierarchy(ctx, communities); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: domain hierarchy generation: %v\n", err)
	}

	graph.LinkCommunitiesToGraph(communities, graphData)

	if err := graph.SaveCommunities(communities, indexCommunitiesPath); err != nil {
		return fmt.Errorf("save communities: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Saved communities to %s\n", indexCommunitiesPath)

	fmt.Fprintf(os.Stderr, "\n✓ Indexing complete!\n")
	fmt.Fprintf(os.Stderr, "  Graph: %s\n", indexGraphPath)
	fmt.Fprintf(os.Stderr, "  Communities: %s\n", indexCommunitiesPath)
	fmt.Fprintf(os.Stderr, "  Nodes: %d, Edges: %d, Communities: %d\n", len(graphData.Nodes), len(graphData.Edges), len(communities))

	return nil
}

func isWorkspaceDir(dir string) bool {
	// Workspace has hash-named directories (8 hex chars)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() && workspace.IsHexHash(e.Name()) {
			return true
		}
	}
	return false
}

func collectWorkspaceChunks(workspaceDir string) ([]graph.ChunkInput, []graph.ChunkWithTopic, error) {
	ws, err := workspace.New(workspaceDir)
	if err != nil {
		return nil, nil, fmt.Errorf("workspace: %w", err)
	}

	docs, err := ws.ListDocuments()
	if err != nil {
		return nil, nil, fmt.Errorf("list documents: %w", err)
	}

	if len(docs) == 0 {
		fmt.Fprintf(os.Stderr, "No documents found in workspace\n")
		return nil, nil, nil
	}

	fmt.Fprintf(os.Stderr, "Found %d documents in workspace\n", len(docs))

	var allChunks []graph.ChunkInput
	var allChunksWithTopic []graph.ChunkWithTopic

	for _, doc := range docs {
		latestPath := filepath.Join(ws.RootPath, doc.Hash, "latest")
		var contentPath string

		if link, err := os.Readlink(latestPath); err == nil {
			contentPath = filepath.Join(ws.RootPath, doc.Hash, "versions", link, "final.md")
		} else {
			versionsDir := filepath.Join(ws.RootPath, doc.Hash, "versions")
			entries, _ := os.ReadDir(versionsDir)
			if len(entries) == 0 {
				continue
			}
			latest := entries[len(entries)-1].Name()
			contentPath = filepath.Join(versionsDir, latest, "final.md")
		}

		content, err := os.ReadFile(contentPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", contentPath, err)
			continue
		}

		chunks := parseChunksFromMarkdown(string(content), doc.Hash)
		for i, ch := range chunks {
			chunkID := fmt.Sprintf("%s:chunk_%d", doc.Hash, i)
			allChunks = append(allChunks, graph.ChunkInput{
				DocID:      doc.Hash,
				ChunkIndex: i,
				Content:    ch.Content,
				Topic:      ch.Topic,
			})
			allChunksWithTopic = append(allChunksWithTopic, graph.ChunkWithTopic{
				ID:        chunkID,
				Topic:     ch.Topic,
				Summary:   ch.Summary,
				KeyPoints: ch.KeyPoints,
				Content:   ch.Content,
			})
		}
	}

	return allChunks, allChunksWithTopic, nil
}

func collectFlatFileChunks(dir string) ([]graph.ChunkInput, []graph.ChunkWithTopic, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read directory: %w", err)
	}

	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".md" && ext != ".markdown" {
			continue
		}
		if strings.HasPrefix(e.Name(), "._") {
			continue
		}
		mdFiles = append(mdFiles, e.Name())
	}

	if len(mdFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No .md files found in %s\n", dir)
		return nil, nil, nil
	}

	fmt.Fprintf(os.Stderr, "Found %d .md files\n", len(mdFiles))

	var allChunks []graph.ChunkInput
	var allChunksWithTopic []graph.ChunkWithTopic

	for _, fileName := range mdFiles {
		filePath := filepath.Join(dir, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", filePath, err)
			continue
		}

		// Parse frontmatter for metadata
		fm := parseFlatFrontmatter(string(content))
		docID := workspace.SourceHash(fileName)

		chunks := parseChunksFromMarkdown(string(content), docID)
		for i, ch := range chunks {
			chunkID := fmt.Sprintf("%s:chunk_%d", docID, i)
			allChunks = append(allChunks, graph.ChunkInput{
				DocID:      docID,
				ChunkIndex: i,
				Content:    ch.Content,
				Topic:      ch.Topic,
			})
			allChunksWithTopic = append(allChunksWithTopic, graph.ChunkWithTopic{
				ID:        chunkID,
				Topic:     ch.Topic,
				Summary:   ch.Summary,
				KeyPoints: ch.KeyPoints,
				Content:   ch.Content,
			})
		}

		_ = fm // metadata available if needed
	}

	return allChunks, allChunksWithTopic, nil
}

func parseFlatFrontmatter(content string) map[string]interface{} {
	if !strings.HasPrefix(content, "---\n") {
		return nil
	}

	endIdx := strings.Index(content[4:], "\n---")
	if endIdx < 0 {
		return nil
	}

	fmText := content[4 : 4+endIdx]
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(fmText), &result); err != nil {
		return nil
	}
	return result
}

type parsedChunk struct {
	Content   string
	Topic     string
	Summary   string
	KeyPoints []string
}

func parseChunksFromMarkdown(content, docHash string) []parsedChunk {
	var chunks []parsedChunk
	lines := strings.Split(content, "\n")
	var currentChunk *parsedChunk
	var inKeyPoints bool

	for _, line := range lines {
		if strings.HasPrefix(line, "## chunk_") {
			if currentChunk != nil && currentChunk.Content != "" {
				chunks = append(chunks, *currentChunk)
			}
			currentChunk = &parsedChunk{}
			inKeyPoints = false
			continue
		}

		if currentChunk == nil {
			continue
		}

		if strings.HasPrefix(line, "topic: ") {
			currentChunk.Topic = strings.TrimPrefix(line, "topic: ")
			inKeyPoints = false
		} else if strings.HasPrefix(line, "summary: ") {
			currentChunk.Summary = strings.TrimPrefix(line, "summary: ")
			inKeyPoints = false
		} else if strings.HasPrefix(line, "key_points:") {
			inKeyPoints = true
		} else if strings.HasPrefix(line, "- ") && inKeyPoints {
			currentChunk.KeyPoints = append(currentChunk.KeyPoints, strings.TrimPrefix(line, "- "))
		} else if line == "" && inKeyPoints {
			inKeyPoints = false
		} else if !strings.HasPrefix(line, "type: ") && !strings.HasPrefix(line, "source_quote: ") &&
			!strings.HasPrefix(line, "my_takeaway: ") && !strings.HasPrefix(line, "confidence: ") &&
			!strings.HasPrefix(line, "applicability: ") && !strings.HasPrefix(line, "steps:") &&
			!strings.HasPrefix(line, "context: ") {
			if currentChunk.Content != "" {
				currentChunk.Content += "\n"
			}
			currentChunk.Content += line
		}
	}

	if currentChunk != nil && currentChunk.Content != "" {
		chunks = append(chunks, *currentChunk)
	}

	if len(chunks) == 0 {
		chunks = append(chunks, parsedChunk{
			Content: content,
			Topic:   "general",
			Summary: "",
		})
	}

	return chunks
}

func detectVectorSize(model string) int {
	switch {
	case strings.Contains(model, "bge-m3"):
		return 1024
	case strings.Contains(model, "qwen3-embedding"):
		return 1024
	case strings.Contains(model, "nomic"):
		return 768
	case strings.Contains(model, "text-embedding-3-small"):
		return 1536
	case strings.Contains(model, "text-embedding-3-large"):
		return 3072
	case strings.Contains(model, "text-embedding-ada"):
		return 1536
	default:
		return 1024 // safe default
	}
}

func stringToUint64(s string) uint64 {
	h := workspace.SourceHash(s)
	var b [8]byte
	for i := 0; i < 8 && i < len(h); i++ {
		b[i] = h[i]
	}
	return binary.BigEndian.Uint64(b[:])
}

func init() {
	indexCmd.Flags().StringVar(&indexQdrantURL, "qdrant-url", "", "Qdrant URL (default: http://localhost:6333)")
	indexCmd.Flags().StringVar(&indexQdrantAPIKey, "qdrant-api-key", "", "Qdrant API key")
	indexCmd.Flags().StringVar(&indexCollection, "collection", "media2rag", "Qdrant collection name")
	indexCmd.Flags().StringVar(&indexGraphPath, "graph-path", "", "Path to save graph.json")
	indexCmd.Flags().StringVar(&indexCommunitiesPath, "communities-path", "", "Path to save communities.json")
	indexCmd.Flags().BoolVar(&indexIncremental, "incremental", false, "Only index new/changed documents")
	indexCmd.Flags().IntVar(&indexConcurrency, "concurrency", 5, "Concurrent LLM requests")
	indexCmd.Flags().StringVar(&indexBackend, "backend", "", "LLM backend")
	indexCmd.Flags().StringVar(&indexModel, "model", "", "LLM model for extraction")
	indexCmd.Flags().StringVar(&indexEmbedModel, "embed-model", "", "Embedding model (defaults to --model)")
	indexCmd.Flags().StringVar(&indexEmbedBackend, "embed-backend", "", "Embedding backend (defaults to --backend)")
	rootCmd.AddCommand(indexCmd)
}
