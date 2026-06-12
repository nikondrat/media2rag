package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	indexClusterMethod   string
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

	// --- Incremental: filter unchanged and already-processed chunks ---
	manifestPath := filepath.Join(filepath.Dir(indexGraphPath), "chunk_manifest.json")
	oldHashes := loadChunkHashes(manifestPath)
	processedChunks := loadProcessedChunks(manifestPath)
	newHashes := make(map[string]string)
	for _, ch := range allChunks {
		chunkID := fmt.Sprintf("%s:chunk_%d", ch.DocID, ch.ChunkIndex)
		newHashes[chunkID] = chunkHash(ch.Content)
	}

	if indexIncremental && len(oldHashes) > 0 {
		var changed []graph.ChunkInput
		var changedWithTopic []graph.ChunkWithTopic
		var unchanged, alreadyProcessed int

		for _, ch := range allChunks {
			chunkID := fmt.Sprintf("%s:chunk_%d", ch.DocID, ch.ChunkIndex)
			if processedChunks[chunkID] {
				alreadyProcessed++
				continue
			}
			if oldHashes[chunkID] == newHashes[chunkID] {
				unchanged++
				continue
			}
			changed = append(changed, ch)
			for _, ct := range allChunksWithTopic {
				if ct.ID == chunkID {
					changedWithTopic = append(changedWithTopic, ct)
					break
				}
			}
		}

		if unchanged > 0 || alreadyProcessed > 0 {
			fmt.Fprintf(os.Stderr, "Incremental: %d unchanged, %d already processed, %d to process\n", unchanged, alreadyProcessed, len(changed))
		}

		allChunks = changed
		allChunksWithTopic = changedWithTopic

		if len(allChunks) == 0 {
			fmt.Fprintf(os.Stderr, "No changes detected. Graph is up to date.\n")
			return nil
		}
	}

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
	checkpointDir := filepath.Join(filepath.Dir(indexGraphPath), "extractions")

	// Pre-filter chunks that already have checkpoints
	checkpointResults, _ := graph.LoadExtractionCheckpoints(checkpointDir)
	var filtered []graph.ChunkInput
	var filteredWithTopic []graph.ChunkWithTopic
	for i, ch := range allChunks {
		chunkID := fmt.Sprintf("%s:chunk_%d", ch.DocID, ch.ChunkIndex)
		if checkpointResults[chunkID] == nil {
			filtered = append(filtered, ch)
			filteredWithTopic = append(filteredWithTopic, allChunksWithTopic[i])
		}
	}
	skipCount := len(allChunks) - len(filtered)
	allChunks = filtered
	allChunksWithTopic = filteredWithTopic

	totalChunks := len(allChunks)
	if skipCount > 0 {
		fmt.Fprintf(os.Stderr, "Found %d existing checkpoints, skipping...\n", skipCount)
	}
	fmt.Fprintf(os.Stderr, "Extracting entities and relations from %d chunks (%d concurrent)...\n", totalChunks, indexConcurrency)

	// Track processed chunks for checkpointing and progress
	var processedMu sync.Mutex
	processedIDs := make([]string, 0)
	processedSet := make(map[string]bool)
	// Start with already-processed IDs from manifest
	for id := range processedChunks {
		processedIDs = append(processedIDs, id)
		processedSet[id] = true
	}

	startTime := time.Now()
	newDone := 0

	extractor := graph.NewEntityExtractorWithCheckpoint(extractClient, indexConcurrency, modelName, checkpointDir)
	results := extractor.ExtractBatchWithCheckpoint(ctx, allChunks, nil, func(ids []string) {
		processedMu.Lock()
		for _, id := range ids {
			if !processedSet[id] {
				processedIDs = append(processedIDs, id)
				processedSet[id] = true
			}
		}
		newDone = len(processedIDs) - len(processedChunks)
		if newDone < 0 {
			newDone = 0
		}
		elapsed := time.Since(startTime)
		remaining := totalChunks - newDone
		if newDone > 0 {
			rate := float64(newDone) / elapsed.Seconds()
			eta := time.Duration(float64(remaining)/rate) * time.Second
			fmt.Fprintf(os.Stderr, "\r  Progress: %d/%d chunks (%.1f chunks/s, ETA: %s)", newDone, totalChunks, rate, eta.Round(time.Second))
		}
		saveChunkHashes(manifestPath, filterHashes(newHashes, processedIDs), processedIDs)
		processedMu.Unlock()
	})
	fmt.Fprintf(os.Stderr, "\n")

	// Final manifest save
	processedMu.Lock()
	for id := range processedChunks {
		if !processedSet[id] {
			processedIDs = append(processedIDs, id)
		}
	}
	processedIDs = removeDups(processedIDs)
	saveChunkHashes(manifestPath, filterHashes(newHashes, processedIDs), processedIDs)
	processedMu.Unlock()

	// Load existing graph for incremental merge
	var graphData *model.KnowledgeGraph
	if indexIncremental && graph.GraphExists(indexGraphPath) {
		fmt.Fprintf(os.Stderr, "Loading existing graph for merge...\n")
		graphData, err = graph.LoadGraph(indexGraphPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load existing graph: %v, starting fresh\n", err)
			graphData = model.NewKnowledgeGraph()
		}
	} else {
		graphData = model.NewKnowledgeGraph()
	}

	// Merge checkpoint results (from previous partial runs)
	for _, cp := range checkpointResults {
		for _, node := range cp.Nodes {
			graphData.AddNode(node)
		}
		for _, edge := range cp.Edges {
			graphData.AddEdge(edge)
		}
	}

	// Merge new results
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
		for _, cp := range checkpointResults {
			for _, edge := range cp.Edges {
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
	fmt.Fprintf(os.Stderr, "Detecting communities (method: %s)...\n", indexClusterMethod)

	var communities []*model.Community
	if indexIncremental && graph.GraphExists(indexCommunitiesPath) {
		communities, err = graph.LoadCommunities(indexCommunitiesPath)
		if err != nil {
			communities = nil
		}
	}

	detector := graph.NewCommunityDetector(extractClient, modelName)

	if indexClusterMethod == "leiden" {
		// Leiden clustering based on graph structure
		leiden := graph.NewLeidenCluster(1.0, 10)
		newCommunities := leiden.Detect(graphData)
		communities = mergeCommunities(communities, newCommunities)
	} else {
		// Topic-based clustering (default)
		newCommunities := detector.DetectGroups(allChunksWithTopic)
		communities = mergeCommunities(communities, newCommunities)
	}
	fmt.Fprintf(os.Stderr, "Total communities: %d\n", len(communities))

	// Only generate summaries for new communities in incremental mode
	if !indexIncremental {
		fmt.Fprintf(os.Stderr, "Generating community summaries...\n")
		if err := detector.GenerateSummaries(ctx, communities, allChunksWithTopic); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: community summary generation: %v\n", err)
		}
	} else {
		// Generate summaries only for communities without them
		var needSummary []*model.Community
		for _, c := range communities {
			if c.Summary == "" {
				needSummary = append(needSummary, c)
			}
		}
		if len(needSummary) > 0 {
			fmt.Fprintf(os.Stderr, "Generating summaries for %d new communities...\n", len(needSummary))
			if err := detector.GenerateSummaries(ctx, needSummary, allChunksWithTopic); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: community summary generation: %v\n", err)
			}
		}
	}

	if err := detector.GenerateDomainHierarchy(ctx, communities); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: domain hierarchy generation: %v\n", err)
	}

	graph.LinkCommunitiesToGraph(communities, graphData)

	if err := graph.SaveCommunities(communities, indexCommunitiesPath); err != nil {
		return fmt.Errorf("save communities: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Saved communities to %s\n", indexCommunitiesPath)

	// Clean up per-chunk checkpoints (all done, no rollback needed)
	if err := os.RemoveAll(checkpointDir); err == nil {
		fmt.Fprintf(os.Stderr, "Cleaned up extraction checkpoints\n")
	}

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
			chunkText := ch.ChunkText()
			allChunks = append(allChunks, graph.ChunkInput{
				DocID:      doc.Hash,
				ChunkIndex: i,
				Content:    chunkText,
				Topic:      ch.Topic,
			})
			allChunksWithTopic = append(allChunksWithTopic, graph.ChunkWithTopic{
				ID:        chunkID,
				Topic:     ch.Topic,
				Summary:   ch.Summary,
				KeyPoints: ch.KeyPoints,
				Content:   chunkText,
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
			chunkText := ch.ChunkText()
			allChunks = append(allChunks, graph.ChunkInput{
				DocID:      docID,
				ChunkIndex: i,
				Content:    chunkText,
				Topic:      ch.Topic,
			})
			allChunksWithTopic = append(allChunksWithTopic, graph.ChunkWithTopic{
				ID:        chunkID,
				Topic:     ch.Topic,
				Summary:   ch.Summary,
				KeyPoints: ch.KeyPoints,
				Content:   chunkText,
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
			if currentChunk != nil && hasChunkData(currentChunk) {
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

	if currentChunk != nil && hasChunkData(currentChunk) {
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

func hasChunkData(ch *parsedChunk) bool {
	return ch.Summary != "" || ch.Content != "" || len(ch.KeyPoints) > 0
}

// ChunkText returns structured text for embedding/extraction
func (ch *parsedChunk) ChunkText() string {
	var sb strings.Builder
	if ch.Topic != "" {
		sb.WriteString("Topic: ")
		sb.WriteString(ch.Topic)
		sb.WriteString("\n")
	}
	if ch.Summary != "" {
		sb.WriteString("Summary: ")
		sb.WriteString(ch.Summary)
		sb.WriteString("\n")
	}
	if len(ch.KeyPoints) > 0 {
		sb.WriteString("Key points: ")
		sb.WriteString(strings.Join(ch.KeyPoints, ", "))
		sb.WriteString("\n")
	}
	if ch.Content != "" {
		sb.WriteString(ch.Content)
	}
	return strings.TrimSpace(sb.String())
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

func chunkHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

type chunkManifest struct {
	Hashes    map[string]string `json:"hashes"`
	Processed []string          `json:"processed,omitempty"`
	Updated   string            `json:"updated"`
}

func loadChunkHashes(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]string)
	}
	var m chunkManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]string)
	}
	return m.Hashes
}

func loadProcessedChunks(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]bool)
	}
	var m chunkManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]bool)
	}
	processed := make(map[string]bool)
	for _, id := range m.Processed {
		processed[id] = true
	}
	return processed
}

func filterHashes(all map[string]string, keepIDs []string) map[string]string {
	filtered := make(map[string]string, len(keepIDs))
	keep := make(map[string]bool, len(keepIDs))
	for _, id := range keepIDs {
		keep[id] = true
	}
	for id, hash := range all {
		if keep[id] {
			filtered[id] = hash
		}
	}
	return filtered
}

func removeDups(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func saveChunkHashes(path string, hashes map[string]string, processed []string) {
	m := chunkManifest{
		Hashes:    hashes,
		Processed: processed,
		Updated:   time.Now().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0644)
}

func mergeCommunities(existing, new []*model.Community) []*model.Community {
	existingMap := make(map[string]*model.Community)
	for _, c := range existing {
		existingMap[c.Topic] = c
	}

	result := make([]*model.Community, len(existing))
	copy(result, existing)

	for _, c := range new {
		if existing, ok := existingMap[c.Topic]; ok {
			existingIDs := make(map[string]bool)
			for _, id := range existing.MemberChunkIDs {
				existingIDs[id] = true
			}
			for _, id := range c.MemberChunkIDs {
				if !existingIDs[id] {
					existing.MemberChunkIDs = append(existing.MemberChunkIDs, id)
				}
			}
		} else {
			result = append(result, c)
		}
	}

	return result
}

func init() {
	indexCmd.Flags().StringVar(&indexQdrantURL, "qdrant-url", "", "Qdrant URL (default: http://localhost:6333)")
	indexCmd.Flags().StringVar(&indexQdrantAPIKey, "qdrant-api-key", "", "Qdrant API key")
	indexCmd.Flags().StringVar(&indexCollection, "collection", "media2rag", "Qdrant collection name")
	indexCmd.Flags().StringVar(&indexGraphPath, "graph-path", "", "Path to save graph.json")
	indexCmd.Flags().StringVar(&indexCommunitiesPath, "communities-path", "", "Path to save communities.json")
	indexCmd.Flags().BoolVar(&indexIncremental, "incremental", true, "Only index new/changed documents (default: true)")
	indexCmd.Flags().IntVar(&indexConcurrency, "concurrency", 5, "Concurrent LLM requests")
	indexCmd.Flags().StringVar(&indexBackend, "backend", "", "LLM backend")
	indexCmd.Flags().StringVar(&indexModel, "model", "", "LLM model for extraction")
	indexCmd.Flags().StringVar(&indexEmbedModel, "embed-model", "", "Embedding model (defaults to --model)")
	indexCmd.Flags().StringVar(&indexEmbedBackend, "embed-backend", "", "Embedding backend (defaults to --backend)")
	indexCmd.Flags().StringVar(&indexClusterMethod, "cluster", "leiden", "Clustering method: leiden (default), topic")
	rootCmd.AddCommand(indexCmd)
}
