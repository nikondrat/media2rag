package graph

import (
	"math/rand"
	"sort"

	"media2rag/internal/model"
)

// LeidenCluster performs Leiden community detection on a knowledge graph
type LeidenCluster struct {
	resolution  float64
	iterations  int
	randomState *rand.Rand
}

// NewLeidenCluster creates a Leiden clustering instance
func NewLeidenCluster(resolution float64, iterations int) *LeidenCluster {
	if resolution <= 0 {
		resolution = 1.0
	}
	if iterations <= 0 {
		iterations = 10
	}
	return &LeidenCluster{
		resolution:  resolution,
		iterations:  iterations,
		randomState: rand.New(rand.NewSource(42)),
	}
}

// Detect finds communities using the Leiden algorithm
func (lc *LeidenCluster) Detect(kg *model.KnowledgeGraph) []*model.Community {
	if len(kg.Nodes) == 0 {
		return nil
	}

	// Build adjacency structure with weights
	nodeIndex := make(map[string]int)
	for i, node := range kg.Nodes {
		nodeIndex[node.ID] = i
	}

	n := len(kg.Nodes)
	adjacency := make([]map[int]float64, n)
	for i := range adjacency {
		adjacency[i] = make(map[int]float64)
	}

	// Build weighted adjacency from edges
	for _, edge := range kg.Edges {
		fromIdx, fromOk := nodeIndex[edge.From]
		toIdx, toOk := nodeIndex[edge.To]
		if !fromOk || !toOk {
			continue
		}
		weight := edge.Confidence
		if weight <= 0 {
			weight = 0.5
		}
		adjacency[fromIdx][toIdx] += weight
		adjacency[toIdx][fromIdx] += weight
	}

	// Initialize: each node in its own community
	community := make([]int, n)
	for i := range community {
		community[i] = i
	}

	// Run Leiden iterations
	for iter := 0; iter < lc.iterations; iter++ {
		// Phase 1: Local moving
		community = lc.localMoving(community, adjacency, n)

		// Phase 2: Refinement
		community = lc.refinement(community, adjacency, n)

		// Phase 3: Aggregation
		community = lc.aggregate(community, adjacency)

		// Check convergence
		if lc.isConverged(community) {
			break
		}
	}

	// Convert to Community model
	return lc.buildCommunities(community, kg)
}

// localMoving moves nodes to neighboring communities to maximize modularity
func (lc *LeidenCluster) localMoving(community []int, adjacency []map[int]float64, n int) []int {
	comm := make([]int, n)
	copy(comm, community)

	totalWeight := 0.0
	for _, edges := range adjacency {
		for _, w := range edges {
			totalWeight += w
		}
	}
	totalWeight /= 2.0

	if totalWeight == 0 {
		return comm
	}

	// Compute node degrees
	degree := make([]float64, n)
	for i, edges := range adjacency {
		for _, w := range edges {
			degree[i] += w
		}
	}

	// Compute community totals
	commTotal := make(map[int]float64)
	commSize := make(map[int]int)
	for i := 0; i < n; i++ {
		commTotal[comm[i]] += degree[i]
		commSize[comm[i]]++
	}

	// Local moving
	improved := true
	maxIter := 10
	for improved && maxIter > 0 {
		improved = false
		maxIter--

		// Random order
		order := lc.randomState.Perm(n)

		for _, i := range order {
			if len(adjacency[i]) == 0 {
				continue
			}

			currentComm := comm[i]

			// Find neighboring communities
			neighborComms := make(map[int]bool)
			neighborComms[currentComm] = true
			for neighbor := range adjacency[i] {
				neighborComms[comm[neighbor]] = true
			}

			// Compute delta modularity for each community
			bestComm := currentComm
			bestDelta := 0.0

			for targetComm := range neighborComms {
				if targetComm == currentComm {
					continue
				}

				// ki_to_target: sum of weights from i to nodes in targetComm
				kiToTarget := 0.0
				for neighbor, w := range adjacency[i] {
					if comm[neighbor] == targetComm {
						kiToTarget += w
					}
				}

				// ki_to_current: sum of weights from i to nodes in currentComm (excluding self)
				kiToCurrent := 0.0
				for neighbor, w := range adjacency[i] {
					if comm[neighbor] == currentComm && neighbor != i {
						kiToCurrent += w
					}
				}

				// Delta modularity
				sigmaTotTarget := commTotal[targetComm]
				sigmaTotCurrent := commTotal[currentComm] - degree[i]

				deltaTarget := kiToTarget - lc.resolution*degree[i]*sigmaTotTarget/(2.0*totalWeight)
				deltaCurrent := kiToCurrent - lc.resolution*degree[i]*sigmaTotCurrent/(2.0*totalWeight)

				delta := deltaTarget - deltaCurrent

				if delta > bestDelta {
					bestDelta = delta
					bestComm = targetComm
				}
			}

			if bestComm != currentComm {
				// Update community totals
				commTotal[currentComm] -= degree[i]
				commSize[currentComm]--
				commTotal[bestComm] += degree[i]
				commSize[bestComm]++

				comm[i] = bestComm
				improved = true
			}
		}
	}

	return comm
}

// refinement ensures communities are well-connected
func (lc *LeidenCluster) refinement(community []int, adjacency []map[int]float64, n int) []int {
	// Group nodes by community
	commNodes := make(map[int][]int)
	for i, c := range community {
		commNodes[c] = append(commNodes[c], i)
	}

	// For each community, check connectivity and split if needed
	refined := make([]int, n)
	copy(refined, community)
	nextComm := 0
	for _, c := range community {
		if _, ok := commNodes[c]; ok {
			nextComm = maxInt(nextComm, c+1)
		}
	}

	for commID, nodes := range commNodes {
		if len(nodes) <= 1 {
			continue
		}

		// Find connected components within community
		components := lc.findConnectedComponents(nodes, adjacency)

		if len(components) > 1 {
			// Split into separate communities
			for _, component := range components {
				for _, node := range component {
					refined[node] = nextComm
				}
				nextComm++
			}
			delete(commNodes, commID)
		}
	}

	return refined
}

// findConnectedComponents finds connected components in a subgraph
func (lc *LeidenCluster) findConnectedComponents(nodes []int, adjacency []map[int]float64) [][]int {
	nodeSet := make(map[int]bool)
	for _, n := range nodes {
		nodeSet[n] = true
	}

	visited := make(map[int]bool)
	var components [][]int

	for _, start := range nodes {
		if visited[start] {
			continue
		}

		// BFS
		var component []int
		queue := []int{start}
		visited[start] = true

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			component = append(component, current)

			for neighbor := range adjacency[current] {
				if nodeSet[neighbor] && !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}

		components = append(components, component)
	}

	return components
}

// aggregate creates a new partition by merging communities
func (lc *LeidenCluster) aggregate(community []int, adjacency []map[int]float64) []int {
	// Renumber communities consecutively
	commMap := make(map[int]int)
	nextComm := 0
	result := make([]int, len(community))

	for _, c := range community {
		if _, ok := commMap[c]; !ok {
			commMap[c] = nextComm
			nextComm++
		}
		result[commMap[c]] = commMap[c]
	}

	for i := range community {
		community[i] = commMap[community[i]]
	}

	return community
}

// isConverged checks if all nodes are in stable communities
func (lc *LeidenCluster) isConverged(community []int) bool {
	// Simple convergence: check if number of unique communities is stable
	unique := make(map[int]bool)
	for _, c := range community {
		unique[c] = true
	}
	return len(unique) <= 1 || len(unique) == len(community)
}

// buildCommunities converts Leiden output to model.Community
func (lc *LeidenCluster) buildCommunities(community []int, kg *model.KnowledgeGraph) []*model.Community {
	// Group nodes by community
	commNodes := make(map[int][]*model.GraphNode)
	for i, commID := range community {
		if i < len(kg.Nodes) {
			commNodes[commID] = append(commNodes[commID], kg.Nodes[i])
		}
	}

	// Convert to Community model
	var communities []*model.Community
	for _, nodes := range commNodes {
		if len(nodes) == 0 {
			continue
		}

		// Generate topic from most common node types/names
		topic := lc.generateTopic(nodes)

		chunkIDs := make([]string, 0)
		chunkSeen := make(map[string]bool)
		for _, node := range nodes {
			for _, chunkID := range node.SourceChunks {
				if !chunkSeen[chunkID] {
					chunkSeen[chunkID] = true
					chunkIDs = append(chunkIDs, chunkID)
				}
			}
		}

		nodeIDs := make([]string, len(nodes))
		for i, node := range nodes {
			nodeIDs[i] = node.ID
		}

		communities = append(communities, &model.Community{
			ID:             model.NodeID(topic, "community"),
			Topic:          topic,
			MemberChunkIDs: chunkIDs,
			MemberNodeIDs:  nodeIDs,
			Domain:         "leiden",
		})
	}

	// Sort by size (largest first)
	sort.Slice(communities, func(i, j int) bool {
		return len(communities[i].MemberChunkIDs) > len(communities[j].MemberChunkIDs)
	})

	return communities
}

// generateTopic generates a topic name from community nodes
func (lc *LeidenCluster) generateTopic(nodes []*model.GraphNode) string {
	if len(nodes) == 0 {
		return "unknown"
	}

	// Count node types
	typeCount := make(map[string]int)
	for _, node := range nodes {
		typeCount[node.Type]++
	}

	// Find most common type
	bestType := ""
	bestCount := 0
	for t, count := range typeCount {
		if count > bestCount {
			bestType = t
			bestCount = count
		}
	}

	// Get representative names
	var names []string
	seen := make(map[string]bool)
	for _, node := range nodes {
		if node.Type == bestType && !seen[node.Name] {
			names = append(names, node.Name)
			seen[node.Name] = true
		}
		if len(names) >= 3 {
			break
		}
	}

	if len(names) == 0 {
		names = append(names, nodes[0].Name)
	}

	topic := names[0]
	if len(names) > 1 {
		topic += " + " + names[1]
	}

	return topic
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
