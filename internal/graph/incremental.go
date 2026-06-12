package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"media2rag/internal/model"
)

// IncrementalUpdater handles real-time graph updates from new documents
type IncrementalUpdater struct {
	store     GraphStore
	extractor *EntityExtractor
	resolver  *EntityResolver
	detector  *CommunityDetector
}

// NewIncrementalUpdater creates a new updater
func NewIncrementalUpdater(store GraphStore, extractor *EntityExtractor, resolver *EntityResolver, detector *CommunityDetector) *IncrementalUpdater {
	return &IncrementalUpdater{
		store:     store,
		extractor: extractor,
		resolver:  resolver,
		detector:  detector,
	}
}

// UpdateResult holds the result of an incremental update
type UpdateResult struct {
	NodesAdded    int
	EdgesAdded    int
	NodesMerged   int
	EdgesMerged   int
	CommunitiesUp int
	TotalNodes    int
	TotalEdges    int
}

// Update processes new chunks and merges them into the existing graph
func (u *IncrementalUpdater) Update(ctx context.Context, graphPath, communitiesPath string, chunks []ChunkInput, chunksWithTopic []ChunkWithTopic) (*UpdateResult, error) {
	result := &UpdateResult{}

	// Load existing graph or create new
	var kg *model.KnowledgeGraph
	if u.store.Exists(graphPath) {
		var err error
		kg, err = u.store.Load(graphPath)
		if err != nil {
			return nil, fmt.Errorf("load existing graph: %w", err)
		}
	} else {
		kg = model.NewKnowledgeGraph()
	}

	// Extract entities and relations from new chunks
	extractResults := u.extractor.ExtractBatch(ctx, chunks, nil)

	// Collect new nodes/edges
	var newNodes []*model.GraphNode
	var newEdges []*model.GraphEdge
	for _, r := range extractResults {
		if r.Err != nil {
			continue
		}
		newNodes = append(newNodes, r.Nodes...)
		newEdges = append(newEdges, r.Edges...)
	}

	// Deduplicate new nodes against existing
	existingIDs := make(map[string]bool)
	for _, node := range kg.Nodes {
		existingIDs[node.ID] = true
	}

	var unseenNodes []*model.GraphNode
	for _, node := range newNodes {
		if existingIDs[node.ID] {
			result.NodesMerged++
			// Merge source chunks into existing node
			for _, n := range kg.Nodes {
				if n.ID == node.ID {
					n.SourceChunks = mergeStringSlices(n.SourceChunks, node.SourceChunks)
					n.Aliases = mergeStringSlices(n.Aliases, node.Aliases)
					break
				}
			}
		} else {
			unseenNodes = append(unseenNodes, node)
		}
	}

	// Resolve entities among new nodes only (avoid O(n²) on full graph)
	if len(unseenNodes) > 1 {
		resolved, err := u.resolver.Resolve(ctx, unseenNodes)
		if err == nil {
			unseenNodes = resolved
		}
	}

	// Merge new nodes into graph
	u.store.Merge(kg, unseenNodes, newEdges)
	result.NodesAdded = len(unseenNodes)
	result.EdgesAdded = len(newEdges)

	// Rebuild indexes and validate
	kg.BuildIndexes()
	if err := kg.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation: %w", err)
	}

	// Save graph
	if err := os.MkdirAll(filepath.Dir(graphPath), 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}
	if err := u.store.Save(kg, graphPath); err != nil {
		return nil, fmt.Errorf("save graph: %w", err)
	}

	result.TotalNodes = len(kg.Nodes)
	result.TotalEdges = len(kg.Edges)

	// Update communities
	if len(chunksWithTopic) > 0 {
		result.CommunitiesUp = u.updateCommunities(ctx, communitiesPath, kg, chunksWithTopic)
	}

	return result, nil
}

func (u *IncrementalUpdater) updateCommunities(ctx context.Context, path string, kg *model.KnowledgeGraph, chunksWithTopic []ChunkWithTopic) int {
	var communities []*model.Community
	if u.store.CommunitiesExists(path) {
		communities, _ = u.store.LoadCommunities(path)
	}

	// Detect new communities from new chunks
	newCommunities := u.detector.DetectGroups(chunksWithTopic)

	// Merge into existing
	communities = mergeCommunitiesGo(communities, newCommunities)

	// Generate summaries for communities without them
	var needSummary []*model.Community
	for _, c := range communities {
		if c.Summary == "" {
			needSummary = append(needSummary, c)
		}
	}
	if len(needSummary) > 0 {
		_ = u.detector.GenerateSummaries(ctx, needSummary, chunksWithTopic)
	}

	// Generate domain hierarchy
	_ = u.detector.GenerateDomainHierarchy(ctx, communities)

	// Link to graph
	LinkCommunitiesToGraph(communities, kg)

	// Save
	_ = u.store.SaveCommunities(communities, path)

	return len(newCommunities)
}

func mergeStringSlices(a, b []string) []string {
	seen := make(map[string]bool)
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			a = append(a, s)
			seen[s] = true
		}
	}
	return a
}

// mergeCommunitiesGo merges communities without cobra dependency
func mergeCommunitiesGo(existing, new []*model.Community) []*model.Community {
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
