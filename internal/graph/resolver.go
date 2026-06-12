package graph

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"media2rag/internal/llm"
	"media2rag/internal/model"
)

// EntityResolver deduplicates entities using embedding similarity + LLM
type EntityResolver struct {
	client          llm.LLMClient
	embedClient     llm.LLMClient
	thresholdHigh   float64 // auto-merge above this (0.85)
	thresholdLow    float64 // LLM resolve between low and high (0.7-0.85)
	model           string
}

// NewEntityResolver creates a new resolver
func NewEntityResolver(client llm.LLMClient, embedClient llm.LLMClient, model string) *EntityResolver {
	return &EntityResolver{
		client:        client,
		embedClient:   embedClient,
		thresholdHigh: 0.85,
		thresholdLow:  0.70,
		model:         model,
	}
}

// Resolve deduplicates nodes by computing embeddings and merging similar ones
func (r *EntityResolver) Resolve(ctx context.Context, nodes []*model.GraphNode) ([]*model.GraphNode, error) {
	if len(nodes) == 0 {
		return nodes, nil
	}

	// Compute embeddings in batches for performance
	batchSize := 100
	var texts []string
	var indices []int
	for i, node := range nodes {
		text := node.Name + " " + node.Description
		texts = append(texts, text)
		indices = append(indices, i)
	}

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		batchIdx := indices[i:end]
		embeddings, err := r.embedClient.EmbedBatch(ctx, batch)
		if err != nil {
			// Fallback to sequential single-embed
			for j, idx := range batchIdx {
				emb, eErr := r.embedClient.Embed(ctx, batch[j])
				if eErr == nil {
					nodes[idx].Embedding = emb
				}
			}
			continue
		}
		for j, idx := range batchIdx {
			if j < len(embeddings) {
				nodes[idx].Embedding = embeddings[j]
			}
		}
	}

	// Group nodes by type for comparison (only compare same-type nodes)
	byType := make(map[string][]*model.GraphNode)
	for _, node := range nodes {
		byType[node.Type] = append(byType[node.Type], node)
	}

	merged := make([]*model.GraphNode, 0, len(nodes))
	mergedIDs := make(map[string]bool)

	totalTypes := len(byType)
	typeIdx := 0
	for _, typeNodes := range byType {
		typeIdx++
		totalPairs := len(typeNodes) * (len(typeNodes) - 1) / 2
		if totalPairs < 0 {
			totalPairs = 0
		}
		for i := 0; i < len(typeNodes); i++ {
			if i%200 == 0 && len(typeNodes) > 500 {
				fmt.Fprintf(os.Stderr, "\r  Dedup %s: %d/%d nodes (%d pairs)...  ", typeNodes[0].Type, i, len(typeNodes), totalPairs)
			}

			if mergedIDs[typeNodes[i].ID] {
				continue
			}

			current := typeNodes[i]
			for j := i + 1; j < len(typeNodes); j++ {
				if mergedIDs[typeNodes[j].ID] {
					continue
				}

				other := typeNodes[j]
				similarity := cosineSimilarity(current.Embedding, other.Embedding)

				if similarity >= r.thresholdHigh {
					// Auto-merge
					current = r.mergeNodes(current, other)
					mergedIDs[other.ID] = true
				} else if similarity >= r.thresholdLow {
					// LLM resolve
					shouldMerge, err := r.llmResolve(ctx, current, other)
					if err != nil {
						continue // Skip on error
					}
					if shouldMerge {
						current = r.mergeNodes(current, other)
						mergedIDs[other.ID] = true
					}
				}
			}

			if !mergedIDs[current.ID] {
				merged = append(merged, current)
			}
		}
		if totalPairs > 0 {
			fmt.Fprintf(os.Stderr, "\r  Dedup %s: %d/%d done (%d pairs, %d nodes → %d)   \n", typeNodes[0].Type, typeIdx, totalTypes, totalPairs, len(typeNodes), len(merged))
		}
	}

	return merged, nil
}

// mergeNodes merges two nodes into one, keeping the first as primary
func (r *EntityResolver) mergeNodes(primary, secondary *model.GraphNode) *model.GraphNode {
	// Add alias if name differs
	if primary.Name != secondary.Name {
		found := false
		for _, a := range primary.Aliases {
			if a == secondary.Name {
				found = true
				break
			}
		}
		if !found {
			primary.Aliases = append(primary.Aliases, secondary.Name)
		}
	}

	// Merge source chunks
	for _, sc := range secondary.SourceChunks {
		found := false
		for _, psc := range primary.SourceChunks {
			if psc == sc {
				found = true
				break
			}
		}
		if !found {
			primary.SourceChunks = append(primary.SourceChunks, sc)
		}
	}

	// Merge metadata (secondary overwrites primary on conflicts)
	for k, v := range secondary.Metadata {
		primary.Metadata[k] = v
	}

	return primary
}

// llmResolve asks LLM if two entities should be merged
func (r *EntityResolver) llmResolve(ctx context.Context, a, b *model.GraphNode) (bool, error) {
	prompt := fmt.Sprintf(`Are these two entities the same thing? Answer YES or NO only.

Entity A: "%s" (type: %s) - %s
Entity B: "%s" (type: %s) - %s

Consider:
- Same concept described in different languages (e.g., "склады" vs "warehouse")
- Synonyms or different phrasings of the same concept
- Different aspects of the same thing

Answer YES if they should be merged, NO if they are distinct.`,
		a.Name, a.Type, a.Description,
		b.Name, b.Type, b.Description)

	resp, err := r.client.Chat(ctx, model.ChatRequest{
		Model: r.model,
		Messages: []model.Message{
			{Role: "system", Content: "You are an entity resolution expert. Determine if two entities refer to the same concept."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return false, fmt.Errorf("LLM entity resolution: %w", err)
	}

	answer := strings.TrimSpace(strings.ToUpper(resp.Message.Content))
	return strings.Contains(answer, "YES"), nil
}

// cosineSimilarity computes cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ResolveQueryEntities resolves user query terms to graph entities
func (r *EntityResolver) ResolveQueryEntities(ctx context.Context, query string, graph *model.KnowledgeGraph) ([]*model.GraphNode, error) {
	prompt := fmt.Sprintf(`Extract entity names from this query that might exist in a knowledge graph.

Return in this exact format:
entity: <first entity name>
entity: <second entity name>

Query: %s`, query)

	resp, err := r.client.Chat(ctx, model.ChatRequest{
		Model: r.model,
		Messages: []model.Message{
			{Role: "system", Content: "Extract entity names from user queries. Use the exact format specified."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("extract query entities: %w", err)
	}

	var entityNames []string
	lines := strings.Split(resp.Message.Content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "entity:") {
			val := strings.TrimPrefix(trimmed, "entity:")
			val = strings.TrimSpace(val)
			if val != "" {
				entityNames = append(entityNames, val)
			}
		}
	}

	// Resolve each name to graph nodes
	var resolved []*model.GraphNode
	for _, name := range entityNames {
		// Direct match
		nodes := graph.GetNodesByName(name)
		if len(nodes) > 0 {
			resolved = append(resolved, nodes...)
			continue
		}

		// Fuzzy match via embedding
		queryEmbed, err := r.embedClient.Embed(ctx, name)
		if err != nil {
			continue
		}

		bestSim := 0.0
		var bestNode *model.GraphNode
		for _, node := range graph.Nodes {
			if len(node.Embedding) == 0 {
				continue
			}
			sim := cosineSimilarity(queryEmbed, node.Embedding)
			if sim > bestSim {
				bestSim = sim
				bestNode = node
			}
		}

		if bestNode != nil && bestSim >= r.thresholdLow {
			resolved = append(resolved, bestNode)
		}
	}

	return resolved, nil
}
