package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/graph"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

var (
	graphragGraphPath      string
	graphragCommunitiesPath string
	graphragDepth          int
	graphragMode           string
	graphragFormat         string
	graphragBackend        string
	graphragModel          string
)

var graphragCmd = &cobra.Command{
	Use:   "graphrag <query>",
	Short: "Search knowledge graph with causal reasoning",
	Long: `Search the knowledge graph using entity fan-out, multi-hop traversal, and community summaries.

Usage:
  media2rag graphrag "почему компании банкротятся"
  media2rag graphrag "как снизить издержки" --depth 3
  media2rag graphrag "какие топ-5 тем в базе" --mode global
  media2rag graphrag "query" --format json`,
	Args: cobra.ExactArgs(1),
	RunE: runGraphRAG,
}

func runGraphRAG(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	query := args[0]

	// Load graph
	gPath := graphragGraphPath
	if gPath == "" {
		home, _ := os.UserHomeDir()
		gPath = filepath.Join(home, ".media2rag", "workspace", "..", "data", "graph.json")
	}

	if !graph.GraphExists(gPath) {
		return fmt.Errorf("graph not found at %s, run: media2rag index", gPath)
	}

	kg, err := graph.LoadGraph(gPath)
	if err != nil {
		return fmt.Errorf("load graph: %w", err)
	}

	// Load communities
	cPath := graphragCommunitiesPath
	if cPath == "" {
		home, _ := os.UserHomeDir()
		cPath = filepath.Join(home, ".media2rag", "workspace", "..", "data", "graph_communities.json")
	}

	var communities []*model.Community
	if graph.GraphExists(cPath) {
		communities, err = graph.LoadCommunities(cPath)
		if err != nil {
			communities = nil // Non-fatal
		}
	}

	// Setup LLM client
	backend := graphragBackend
	if backend == "" {
		backend = cfg.LLM.DefaultBackend
	}
	modelName := graphragModel
	if modelName == "" {
		modelName = cfg.LLM.Model
	}

	llmClient, err := llm.NewClient(
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
		return fmt.Errorf("init LLM client: %w", err)
	}

	// Query Rewriter
	rewriter := graph.NewQueryRewriter(llmClient, modelName)
	gq, err := rewriter.Rewrite(ctx, query)
	if err != nil {
		return fmt.Errorf("rewrite query: %w", err)
	}

	// Override mode if specified
	if graphragMode != "" && graphragMode != "auto" {
		gq.Mode = graphragMode
	}
	if graphragDepth > 0 {
		gq.Depth = graphragDepth
	}

	// Execute search based on mode
	var result *model.GraphRAGResult

	switch gq.Mode {
	case model.ModeGlobal:
		result = globalSearch(ctx, query, gq, kg, communities, llmClient, modelName)
	case model.ModeDRIFT:
		result = driftSearch(ctx, query, gq, kg, communities, llmClient, modelName)
	default:
		result = localSearch(ctx, query, gq, kg, llmClient, modelName)
	}

	// Output
	switch graphragFormat {
	case "json":
		return outputGraphRAGJSON(result)
	default:
		return outputGraphRAGText(result)
	}
}

// localSearch performs entity fan-out with multi-hop traversal
func localSearch(ctx context.Context, query string, gq *model.GraphQuery, kg *model.KnowledgeGraph, client llm.LLMClient, modelName string) *model.GraphRAGResult {
	result := &model.GraphRAGResult{
		Query:   query,
		Entities: gq.Entities,
		Pattern: gq.Pattern,
		Mode:    gq.Mode,
	}

	// Find seed entities
	seedNodes := findSeedEntities(gq.Entities, kg)
	if len(seedNodes) == 0 {
		result.Answer = "No matching entities found in the graph."
		return result
	}

	// Traverse graph from seed entities
	chains := traverseGraph(kg, seedNodes, gq.Pattern, gq.Depth, gq.Relations)
	result.Chains = chains

	// Collect provenance
	provenance := make(map[string]bool)
	for _, chain := range chains {
		for _, sc := range chain.SourceChunks {
			provenance[sc] = true
		}
	}
	for sc := range provenance {
		result.Provenance = append(result.Provenance, sc)
	}

	// Generate LLM answer
	if len(chains) > 0 {
		answer, err := generateAnswer(ctx, query, gq.Pattern, chains, client, modelName)
		if err != nil {
			result.Answer = fmt.Sprintf("Error generating answer: %v", err)
		} else {
			result.Answer = answer
		}
	}

	return result
}

// globalSearch uses community summaries for holistic queries
func globalSearch(ctx context.Context, query string, gq *model.GraphQuery, kg *model.KnowledgeGraph, communities []*model.Community, client llm.LLMClient, modelName string) *model.GraphRAGResult {
	result := &model.GraphRAGResult{
		Query:   query,
		Entities: gq.Entities,
		Pattern: gq.Pattern,
		Mode:    model.ModeGlobal,
	}

	if len(communities) == 0 {
		result.Answer = "No communities available. Run: media2rag index"
		return result
	}

	// Rank communities by relevance to query
	type scoredCommunity struct {
		*model.Community
		Score float64
	}

	var scored []scoredCommunity
	for _, c := range communities {
		score := communityRelevance(query, c)
		scored = append(scored, scoredCommunity{Community: c, Score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Take top communities
	topN := 5
	if len(scored) < topN {
		topN = len(scored)
	}

	var contextText strings.Builder
	for i := 0; i < topN; i++ {
		c := scored[i]
		contextText.WriteString(fmt.Sprintf("Community: %s (domain: %s)\n", c.Topic, c.Domain))
		if c.Summary != "" {
			contextText.WriteString(fmt.Sprintf("Summary: %s\n\n", c.Summary))
		}
		result.Provenance = append(result.Provenance, c.MemberChunkIDs...)
	}

	// Generate answer from community summaries
	answer, err := generateAnswerFromContext(ctx, query, contextText.String(), client, modelName)
	if err != nil {
		result.Answer = fmt.Sprintf("Error generating answer: %v", err)
	} else {
		result.Answer = answer
	}

	return result
}

// driftSearch combines local entity search with community context
func driftSearch(ctx context.Context, query string, gq *model.GraphQuery, kg *model.KnowledgeGraph, communities []*model.Community, client llm.LLMClient, modelName string) *model.GraphRAGResult {
	result := &model.GraphRAGResult{
		Query:   query,
		Entities: gq.Entities,
		Pattern: gq.Pattern,
		Mode:    model.ModeDRIFT,
	}

	// Local search
	seedNodes := findSeedEntities(gq.Entities, kg)
	if len(seedNodes) > 0 {
		chains := traverseGraph(kg, seedNodes, gq.Pattern, gq.Depth, gq.Relations)
		result.Chains = chains

		provenance := make(map[string]bool)
		for _, chain := range chains {
			for _, sc := range chain.SourceChunks {
				provenance[sc] = true
			}
		}
		for sc := range provenance {
			result.Provenance = append(result.Provenance, sc)
		}
	}

	// Add community context for entities
	var communityContext strings.Builder
	for _, node := range seedNodes {
		for _, c := range communities {
			for _, nid := range c.MemberNodeIDs {
				if nid == node.ID {
					communityContext.WriteString(fmt.Sprintf("Community context for '%s' (%s):\n", node.Name, c.Topic))
					if c.Summary != "" {
						communityContext.WriteString(c.Summary + "\n\n")
					}
					break
				}
			}
		}
	}

	// Generate answer with both chains and community context
	if len(result.Chains) > 0 || communityContext.Len() > 0 {
		answer, err := generateDRIFTAnswer(ctx, query, result.Chains, communityContext.String(), client, modelName)
		if err != nil {
			result.Answer = fmt.Sprintf("Error generating answer: %v", err)
		} else {
			result.Answer = answer
		}
	}

	return result
}

// findSeedEntities finds graph nodes matching query entities
func findSeedEntities(entities []string, kg *model.KnowledgeGraph) []*model.GraphNode {
	var seeds []*model.GraphNode
	seen := make(map[string]bool)

	for _, entity := range entities {
		// Direct match
		nodes := kg.GetNodesByName(entity)
		for _, n := range nodes {
			if !seen[n.ID] {
				seeds = append(seeds, n)
				seen[n.ID] = true
			}
		}

		// Case-insensitive match on name and aliases
		for _, node := range kg.Nodes {
			if seen[node.ID] {
				continue
			}
			if strings.EqualFold(node.Name, entity) {
				seeds = append(seeds, node)
				seen[node.ID] = true
				continue
			}
			for _, alias := range node.Aliases {
				if strings.EqualFold(alias, entity) {
					seeds = append(seeds, node)
					seen[node.ID] = true
					break
				}
			}
		}
	}

	return seeds
}

// traverseGraph performs multi-hop traversal from seed nodes
func traverseGraph(kg *model.KnowledgeGraph, seeds []*model.GraphNode, pattern string, depth int, relations []string) []model.ReasoningChain {
	var chains []model.ReasoningChain

	for _, seed := range seeds {
		// BFS traversal
		type queueItem struct {
			nodeID   string
			path     []string
			rels     []string
			mechs    []string
			chunks   []string
			conf     float64
		}

		queue := []queueItem{{
			nodeID: seed.ID,
			path:   []string{seed.Name},
			conf:   1.0,
		}}

		visited := make(map[string]bool)
		visited[seed.ID] = true

		for len(queue) > 0 && len(chains) < 10 {
			current := queue[0]
			queue = queue[1:]

			if len(current.path) > depth+1 {
				continue
			}

			// Get edges from current node
			var nextEdges []*model.GraphEdge
			if pattern == model.PatternRootCause || pattern == model.PatternPrerequisites {
				nextEdges = kg.GetInEdges(current.nodeID)
			} else {
				nextEdges = kg.GetOutEdges(current.nodeID)
			}

			if len(nextEdges) == 0 && len(current.path) > 1 {
				// End of path, save chain
				chains = append(chains, model.ReasoningChain{
					Path:         current.path,
					Relations:    current.rels,
					Confidence:   current.conf,
					Mechanisms:   current.mechs,
					SourceChunks: current.chunks,
				})
				continue
			}

			for _, edge := range nextEdges {
				nextID := edge.To
				if pattern == model.PatternRootCause || pattern == model.PatternPrerequisites {
					nextID = edge.From
				}

				if visited[nextID] {
					continue
				}

				nextNode, ok := kg.GetNodeByID(nextID)
				if !ok {
					continue
				}

				newPath := make([]string, len(current.path))
				copy(newPath, current.path)
				newPath = append(newPath, nextNode.Name)

				newRels := make([]string, len(current.rels))
				copy(newRels, current.rels)
				newRels = append(newRels, edge.RelationType)

				newMechs := make([]string, len(current.mechs))
				copy(newMechs, current.mechs)
				if edge.Mechanism != "" {
					newMechs = append(newMechs, edge.Mechanism)
				}

				newChunks := make([]string, len(current.chunks))
				copy(newChunks, current.chunks)
				if edge.SourceChunk != "" {
					newChunks = append(newChunks, edge.SourceChunk)
				}

				newConf := current.conf * edge.Confidence

				queue = append(queue, queueItem{
					nodeID: nextID,
					path:   newPath,
					rels:   newRels,
					mechs:  newMechs,
					chunks: newChunks,
					conf:   newConf,
				})

				visited[nextID] = true
			}
		}
	}

	// Sort chains by confidence
	sort.Slice(chains, func(i, j int) bool {
		return chains[i].Confidence > chains[j].Confidence
	})

	return chains
}

func filterEdges(edges []*model.GraphEdge, allowed []string, relations []string) []*model.GraphEdge {
	if len(allowed) == 0 && len(relations) == 0 {
		return edges
	}

	allowedMap := make(map[string]bool)
	for _, r := range allowed {
		allowedMap[r] = true
	}
	for _, r := range relations {
		allowedMap[r] = true
	}

	var filtered []*model.GraphEdge
	for _, e := range edges {
		if allowedMap[e.RelationType] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func communityRelevance(query string, c *model.Community) float64 {
	query = strings.ToLower(query)
	topic := strings.ToLower(c.Topic)

	// Simple keyword matching
	queryWords := strings.Fields(query)
	score := 0.0

	if strings.Contains(topic, query) || strings.Contains(query, topic) {
		score += 10.0
	}

	for _, word := range queryWords {
		if len(word) < 3 {
			continue
		}
		if strings.Contains(topic, word) {
			score += 1.0
		}
		if strings.Contains(strings.ToLower(c.Summary), word) {
			score += 0.5
		}
	}

	return score
}

func generateAnswer(ctx context.Context, query, pattern string, chains []model.ReasoningChain, client llm.LLMClient, modelName string) (string, error) {
	var chainsText strings.Builder
	for i, chain := range chains {
		chainsText.WriteString(fmt.Sprintf("Chain %d: %s\n", i+1, strings.Join(chain.Path, " → ")))
		if len(chain.Mechanisms) > 0 {
			chainsText.WriteString(fmt.Sprintf("  Mechanisms: %s\n", strings.Join(chain.Mechanisms, "; ")))
		}
		chainsText.WriteString(fmt.Sprintf("  Confidence: %.2f\n\n", chain.Confidence))
	}

	prompt := fmt.Sprintf(`Answer the user's query based on the following reasoning chains from a knowledge graph.

Query: %s
Pattern: %s

Reasoning Chains:
%s

Provide a clear, concise answer that explains the causal relationships. Reference the chains where relevant.

Answer:`, query, pattern, chainsText.String())

	resp, err := client.Chat(ctx, model.ChatRequest{
		Model: modelName,
		Messages: []model.Message{
			{Role: "system", Content: "You are an AI assistant that answers questions based on knowledge graph reasoning chains."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Message.Content), nil
}

func generateAnswerFromContext(ctx context.Context, query, contextText string, client llm.LLMClient, modelName string) (string, error) {
	prompt := fmt.Sprintf(`Answer the user's query based on the following community summaries.

Query: %s

Community Context:
%s

Provide a clear, concise answer that synthesizes insights from the communities.

Answer:`, query, contextText)

	resp, err := client.Chat(ctx, model.ChatRequest{
		Model: modelName,
		Messages: []model.Message{
			{Role: "system", Content: "You are an AI assistant that answers questions based on community summaries."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Message.Content), nil
}

func generateDRIFTAnswer(ctx context.Context, query string, chains []model.ReasoningChain, communityContext string, client llm.LLMClient, modelName string) (string, error) {
	var chainsText strings.Builder
	for i, chain := range chains {
		chainsText.WriteString(fmt.Sprintf("Chain %d: %s\n", i+1, strings.Join(chain.Path, " → ")))
	}

	prompt := fmt.Sprintf(`Answer the user's query based on knowledge graph chains and community context.

Query: %s

Reasoning Chains:
%s

Community Context:
%s

Provide a comprehensive answer combining local graph traversal with broader community context.

Answer:`, query, chainsText.String(), communityContext)

	resp, err := client.Chat(ctx, model.ChatRequest{
		Model: modelName,
		Messages: []model.Message{
			{Role: "system", Content: "You are an AI assistant that answers questions using both local graph traversal and community context."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Message.Content), nil
}

func outputGraphRAGJSON(result *model.GraphRAGResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func outputGraphRAGText(result *model.GraphRAGResult) error {
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Pattern: %s | Mode: %s\n\n", result.Pattern, result.Mode)

	if len(result.Chains) > 0 {
		fmt.Printf("Found %d reasoning chains:\n\n", len(result.Chains))
		for i, chain := range result.Chains {
			fmt.Printf("Chain %d (confidence: %.2f):\n", i+1, chain.Confidence)
			fmt.Printf("  %s\n", strings.Join(chain.Path, " → "))
			if len(chain.Mechanisms) > 0 {
				fmt.Printf("  Mechanisms: %s\n", strings.Join(chain.Mechanisms, "; "))
			}
			fmt.Println()
		}
	}

	fmt.Println("---")
	fmt.Println(result.Answer)

	if len(result.Provenance) > 0 {
		fmt.Printf("\nProvenance: %d source chunks\n", len(result.Provenance))
	}

	return nil
}

func init() {
	home, _ := os.UserHomeDir()
	defaultGraphPath := filepath.Join(home, ".media2rag", "data", "graph.json")
	defaultCommunitiesPath := filepath.Join(home, ".media2rag", "data", "graph_communities.json")

	graphragCmd.Flags().StringVar(&graphragGraphPath, "graph-path", defaultGraphPath, "Path to graph.json")
	graphragCmd.Flags().StringVar(&graphragCommunitiesPath, "communities-path", defaultCommunitiesPath, "Path to communities.json")
	graphragCmd.Flags().IntVar(&graphragDepth, "depth", 0, "Traversal depth (default: auto)")
	graphragCmd.Flags().StringVar(&graphragMode, "mode", "auto", "Search mode: local, global, drift, auto")
	graphragCmd.Flags().StringVar(&graphragFormat, "format", "text", "Output format: text, json")
	graphragCmd.Flags().StringVar(&graphragBackend, "backend", "", "LLM backend")
	graphragCmd.Flags().StringVar(&graphragModel, "model", "", "LLM model")
	rootCmd.AddCommand(graphragCmd)
}
